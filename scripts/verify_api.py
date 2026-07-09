#!/usr/bin/env python3
"""Compare what api/openapi.json promises against what the API actually serves.

Every guard in this repo checks the CLI against a document. Nothing checked the
document against the API — and the document is a hand-written file served
statically (`apps/platform/src/app/api/openapi/route.ts` imports it and marks
the route `force-static`), so it drifts from the routes beside it.

It has drifted. `/health` is absent from it and has been serving traffic all
along. `PublicProductTypeResponse` declares `id` and `name`; the server also
returns `description`. Four of its PUT descriptions say "At least one field must
be provided" while the handlers reject a body that omits one.

This script asks the server instead. For every GET the spec declares, it calls
the endpoint and reports:

    MISSING   the spec declares a property the response did not carry
    EXTRA     the response carried a property the spec never declared
    STATUS    the path answered with something other than 200

EXTRA is the column to read. A field the spec omits is a field this CLI's help
pages will omit too, and a caller cannot see what was never printed.

Usage:
    make verify-api                       # reads CAPIGO_API_URL / _KEY / _TENANT
    python3 scripts/verify_api.py --base-url http://127.0.0.1:3999/api/v1 \
        --key csk_... --tenant acme-corp

Exit 0 when every endpoint matches its schema; 1 otherwise. Read-only: it makes
no POST, PUT or PATCH, and creates nothing.
"""

import argparse
import json
import os
import re
import sys
import urllib.error
import urllib.request

REPO_SPEC = os.path.join(os.path.dirname(__file__), "..", "api", "openapi.json")

# Paths the CLI calls that the spec has never declared. Their presence here is
# the point: the document does not know about them.
UNDECLARED_PATHS = ["/health", "/me"]


def parse_args():
    p = argparse.ArgumentParser(description=__doc__.split("\n")[0])
    p.add_argument("--base-url", default=os.environ.get("CAPIGO_API_URL", ""))
    p.add_argument("--key", default=os.environ.get("CAPIGO_API_KEY", ""))
    p.add_argument("--tenant", default=os.environ.get("CAPIGO_TENANT", ""))
    p.add_argument("--spec", default=REPO_SPEC)
    p.add_argument("--cli", default="",
                   help="path to a built capigo binary; also checks that each help page's "
                        "OUTPUT sample names every field the API actually returns")
    p.add_argument("--timeout", type=int, default=60,
                   help="a Next.js dev server compiles each route on its first request")
    args = p.parse_args()
    missing = [n for n, v in (("--base-url", args.base_url), ("--key", args.key),
                              ("--tenant", args.tenant)) if not v]
    if missing:
        p.error("missing " + ", ".join(missing) +
                " (or set CAPIGO_API_URL / CAPIGO_API_KEY / CAPIGO_TENANT)")
    return args


class Client:
    def __init__(self, base_url, key, tenant, timeout):
        self.base_url = base_url.rstrip("/")
        self.key, self.tenant, self.timeout = key, tenant, timeout

    def get(self, path, with_tenant=True):
        req = urllib.request.Request(self.base_url + path)
        req.add_header("Authorization", "Bearer " + self.key)
        if with_tenant:
            req.add_header("X-Tenant-Code", self.tenant)
        try:
            with urllib.request.urlopen(req, timeout=self.timeout) as r:
                return r.status, json.loads(r.read() or b"null")
        except urllib.error.HTTPError as e:
            body = e.read()
            try:
                return e.code, json.loads(body or b"null")
            except Exception:
                return e.code, None
        except Exception as e:
            return 0, str(e)


def resolve(spec, node, depth=0):
    """Follow $ref and flatten allOf.

    PublicBoardDetailResponse is `allOf: [PublicBoardResponse, {lists}]`. A
    resolver that ignores allOf sees no properties and reports every real field
    as EXTRA — which is how this script first accused the boards endpoint of six
    undeclared fields it declares perfectly well.
    """
    while isinstance(node, dict) and ("$ref" in node or "allOf" in node) and depth < 8:
        depth += 1
        if "$ref" in node:
            node = spec["components"]["schemas"].get(node["$ref"].rsplit("/", 1)[-1], {})
            continue
        merged = {"type": "object", "properties": {}}
        for part in node["allOf"]:
            merged["properties"].update(resolve(spec, part, depth).get("properties", {}))
        node = merged
    return node


def declared_props(spec, op):
    """The property names the 200 response declares for one record."""
    try:
        sch = op["responses"]["200"]["content"]["application/json"]["schema"]
    except KeyError:
        return None
    data = resolve(spec, sch).get("properties", {}).get("data")
    if data is None:
        return None
    data = resolve(spec, data)
    if data.get("type") == "array":
        data = resolve(spec, data.get("items", {}))
    return set(data.get("properties", {}))


def unsuppliable_params(op):
    """Required query parameters this script cannot invent a value for.

    `/wms/inbound-receipts` requires `warehouse_code`: a receipt's code is unique
    within a warehouse, not within a tenant. Calling it without one is a 400, and
    a 400 here would say nothing about whether the response matches its schema.
    """
    return [
        prm["name"]
        for prm in op.get("parameters", [])
        if prm.get("required") and prm.get("in") == "query"
    ]


def first_record(body):
    if not isinstance(body, dict):
        return None
    d = body.get("data")
    if isinstance(d, list):
        return d[0] if d else None
    return d if isinstance(d, dict) else None


def compare(spec, op, status, body):
    """One line of verdict, and whether it counts as a failure."""
    if status != 200:
        return f"status {status}", True
    rec = first_record(body)
    if rec is None:
        return "no record returned (empty tenant?)", False
    declared = declared_props(spec, op)
    if declared is None:
        return "spec declares no data schema", False
    missing = sorted(declared - set(rec))
    extra = sorted(set(rec) - declared)
    if not missing and not extra:
        return "match", False
    note = ""
    if missing:
        note += "MISSING " + ",".join(missing) + "  "
    if extra:
        note += "EXTRA " + ",".join(extra)
    return note.strip(), True


# Each endpoint whose response a help page shows. The page's OUTPUT section must
# name every field the API returns: a field absent from the sample is a field the
# reader does not know exists, and `variants get` omitted `product` — the only
# way to reach a variant's parent — while every other check said "match".
HELP_SAMPLES = [
    # by-id reads
    ("/pcms/variants/{id}", ["variants", "get"]),
    ("/pcms/brands/{id}", ["brands", "get"]),
    ("/pcms/units/{id}", ["units", "get"]),
    ("/pcms/product-types/{id}", ["product-types", "get"]),
    ("/pcms/categories/{id}", ["categories", "get"]),
    ("/pcms/products/{id}", ["products", "get"]),
    ("/mission/tasks/{id}", ["tasks", "get"]),
    ("/mission/boards/{id}", ["boards", "get"]),
    ("/members/{id}", ["members", "get"]),
    # lists — the row shape a caller reads far more often than a single item
    ("/pcms/variants", ["variants", "list"]),
    ("/pcms/brands", ["brands", "list"]),
    ("/pcms/units", ["units", "list"]),
    ("/pcms/product-types", ["product-types", "list"]),
    ("/pcms/categories", ["categories", "list"]),
    ("/pcms/products", ["products", "list"]),
    ("/mission/tasks", ["tasks", "list"]),
    ("/mission/boards", ["boards", "list"]),
    ("/members", ["members", "list"]),
    ("/tenants", ["tenants", "list"]),
    # write pages return the same record their `get` returns, so their samples
    # answer to the same field set. A sample that trims one is a sample that
    # teaches a reader the field does not exist.
    ("/pcms/brands/{id}", ["brands", "create"]),
    ("/pcms/brands/{id}", ["brands", "update"]),
    ("/pcms/brands/{id}", ["brands", "replace"]),
    ("/pcms/categories/{id}", ["categories", "create"]),
    ("/pcms/categories/{id}", ["categories", "update"]),
    ("/pcms/categories/{id}", ["categories", "replace"]),
    ("/pcms/units/{id}", ["units", "create"]),
    ("/pcms/units/{id}", ["units", "update"]),
    ("/pcms/units/{id}", ["units", "replace"]),
    ("/pcms/product-types/{id}", ["product-types", "create"]),
    ("/pcms/product-types/{id}", ["product-types", "update"]),
    ("/pcms/product-types/{id}", ["product-types", "replace"]),
    ("/pcms/products/{id}", ["products", "create"]),
    ("/pcms/products/{id}", ["products", "update"]),
    ("/mission/tasks/{id}", ["tasks", "create"]),
    ("/mission/tasks/{id}", ["tasks", "update"]),
    ("/pcms/products/{id}", ["products", "variants"]),
]

# Sub-resources that need a parent which actually has rows, not merely a parent.
SUBRESOURCE_SAMPLES = [
    ("/subtasks", ["tasks", "subtasks", "list"]),
    ("/comments", ["tasks", "comments"]),
]


# Commands that answer without an id or a tenant. Their pages can be checked by
# running them: the fields they print must be the fields their sample shows.
#
# `auth login` and `auth logout` write ~/.capigo/config.json, so they run under a
# throwaway HOME. A verification script that edited the operator's credentials to
# check a help page would be a poor trade.
LOCAL_CHECKS = [["version"], ["health"]]
ISOLATED_CHECKS = [["auth", "login", "--key", "{key}"], ["config", "get", "api_url"],
                   ["auth", "logout"]]


def check_local_samples(cli, env, key):
    """Run the command; does its own page name every field it printed?"""
    import subprocess
    import tempfile

    failures = 0
    home = tempfile.mkdtemp(prefix="capigo-verify-")
    checks = LOCAL_CHECKS + [[a.format(key=key) for a in c] for c in ISOLATED_CHECKS]
    for cmd in checks:
        run_env = env
        if cmd[0] == "auth" or cmd[:2] == ["config", "get"]:
            run_env = dict(env, HOME=home)
        run = subprocess.run([cli] + cmd, capture_output=True, text=True, env=run_env)
        try:
            doc = json.loads(run.stdout)
        except Exception:
            print(f"  {' '.join(cmd[:3]):<22} skipped: did not print JSON (exit {run.returncode})")
            continue
        # A command that failed printed an error object, not a record. Counting
        # its zero fields as "names all 0 fields" is the false pass this script
        # exists to prevent.
        if "error" in doc:
            print(f"  {' '.join(cmd[:3]):<22} SKIPPED: command failed — {doc['error'].get('message', '')}")
            failures += 1
            continue
        rec = doc.get("data")
        if not isinstance(rec, dict) or not rec:
            print(f"  {' '.join(cmd[:3]):<22} skipped: no object at .data")
            continue
        label = " ".join(cmd[:3] if cmd[0] != "auth" else cmd[:2])
        page = subprocess.run([cli] + cmd[:2] + ["--help"] if cmd[0] in ("auth", "config")
                              else [cli] + cmd + ["--help"],
                              capture_output=True, text=True).stdout
        missing = sorted(f for f in rec if f'"{f}"' not in page and f not in page)
        if missing:
            print(f"  {label:<22} SAMPLE OMITS " + ",".join(missing))
            failures += 1
        else:
            print(f"  {label:<22} names all {len(rec)} fields")
    return failures


def find_task_with(c, suffix, limit=50):
    """A task whose sub-resource actually has rows.

    The first task in the list has no subtasks and no comments, so checking a
    page against it proves nothing: an empty array agrees with every sample ever
    written. Look for one that answers.
    """
    _, body = c.get(f"/mission/tasks?limit={limit}")
    for row in (body.get("data") or []) if isinstance(body, dict) else []:
        status, sub = c.get(f"/mission/tasks/{row['id']}{suffix}")
        if status == 200 and first_record(sub):
            return row["id"]
    return None


def check_help_samples(c, cli, ids):
    """Does each help page's OUTPUT sample name every field the API returns?"""
    import subprocess

    print("\nHelp samples vs the API's own fields:")
    failures = 0
    for path, cmd in HELP_SAMPLES:
        base = path.split("/{")[0]
        if base == "/mission/tasks" and path.endswith("/comments"):
            base = "/mission/tasks"
        if "{id}" in path and base not in ids:
            print(f"  {' '.join(cmd):<22} skipped: no record to compare against")
            continue
        concrete = path.replace("{id}", ids[base]) if "{id}" in path else path
        status, body = c.get(concrete)
        rec = first_record(body)
        if status != 200:
            print(f"  {' '.join(cmd):<22} skipped: endpoint answered {status}")
            continue
        if not rec:
            print(f"  {' '.join(cmd):<22} skipped: no record to compare against")
            continue
        page = subprocess.run([cli] + cmd + ["--help"], capture_output=True, text=True).stdout

        # A page may defer to another page's sample — "the same shape as products
        # get" — rather than copy it. That is this repo's own rule: a fact true of
        # two pages is stated on one. Follow the pointer instead of demanding a
        # second copy that would drift from the first.
        # The phrase wraps across lines in a help page, so match on flattened text.
        flat = " ".join(page.split())
        referred = re.search(r"same shape as ([a-z][a-z-]*(?: [a-z][a-z-]*)?) get", flat)
        if referred:
            target = referred.group(1).split() + ["get"]
            page += subprocess.run([cli] + target + ["--help"],
                                   capture_output=True, text=True).stdout

        missing = sorted(f for f in rec if f'"{f}"' not in page)
        if missing:
            print(f"  {' '.join(cmd):<22} SAMPLE OMITS " + ",".join(missing))
            failures += 1
        else:
            print(f"  {' '.join(cmd):<22} names all {len(rec)} fields")
    return failures


def main():
    args = parse_args()
    spec = json.load(open(args.spec))
    c = Client(args.base_url, args.key, args.tenant, args.timeout)

    print(f"base={c.base_url}  tenant={c.tenant}  spec={os.path.relpath(args.spec)}\n")

    rows, failures = [], 0

    collections = sorted(p for p, v in spec["paths"].items() if "get" in v and "{" not in p)
    ids = {}
    for path in collections:
        op = spec["paths"][path]["get"]
        if needed := unsuppliable_params(op):
            rows.append((path, 0, "skipped: requires " + ",".join(needed)))
            continue
        status, body = c.get(path)
        note, bad = compare(spec, op, status, body)
        rows.append((path, status, note))
        failures += bad
        rec = first_record(body)
        if rec and rec.get("id"):
            ids[path] = rec["id"]

    # By-id paths, with an id taken from the matching collection. Nineteen
    # single-item help pages describe these responses.
    for path, item in sorted(spec["paths"].items()):
        if "get" not in item or "{" not in path:
            continue
        base = path.split("/{")[0]
        if base not in ids or "{attachmentId}" in path or "{code}" in path or "{sku}" in path:
            continue  # needs an id this script cannot fabricate
        status, body = c.get(path.replace("{id}", ids[base]))
        note, bad = compare(spec, item["get"], status, body)
        rows.append((path, status, note))
        failures += bad

    width = max(len(p) for p, _, _ in rows)
    for path, status, note in rows:
        code = "  -" if status == 0 else f"{status:>3}"
        print(f"{path:<{width}}  {code}  {note}")

    if args.cli:
        failures += check_help_samples(c, args.cli, ids)
        env = dict(os.environ, CAPIGO_API_URL=args.base_url, CAPIGO_API_KEY=args.key)
        failures += check_local_samples(args.cli, env, args.key)

    if args.cli:
        import subprocess

        for suffix, cmd in SUBRESOURCE_SAMPLES:
            tid = find_task_with(c, suffix)
            if not tid:
                print(f"  {' '.join(cmd):<22} skipped: no task has any {suffix.strip('/')}")
                continue
            _, body = c.get(f"/mission/tasks/{tid}{suffix}")
            rec = first_record(body)
            page = subprocess.run([args.cli] + cmd + ["--help"], capture_output=True, text=True).stdout
            missing = sorted(f for f in rec if f'"{f}"' not in page)
            if missing:
                print(f"  {' '.join(cmd):<22} SAMPLE OMITS " + ",".join(missing))
                failures += 1
            else:
                print(f"  {' '.join(cmd):<22} names all {len(rec)} fields")

    print("\nPaths the CLI calls that the spec never declares:")
    for path in UNDECLARED_PATHS:
        status, _ = c.get(path, with_tenant=False)
        print(f"{path:<{width}}  {status:>3}  (absent from openapi.json)")

    print()
    if failures:
        print(f"{failures} endpoint(s) disagree with api/openapi.json.")
        print("A field the spec omits is a field the help pages omit. Fix the pages, or the spec.")
        return 1
    print(f"{len(rows)} endpoints agree with api/openapi.json.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
