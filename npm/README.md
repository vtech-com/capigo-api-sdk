# @vtech-com/capigo-skill

One-command installer for the **capigo-api** agent skill — a self-contained guide
(`SKILL.md` + `references/`) that teaches an AI agent how to drive the
[`capigo` CLI](https://github.com/vtech-com/capigo-api-sdk): auth, tenant handling, exit codes,
output modes, and the PCMS catalogue workflows (Product Code / Barcode generation, Brands,
Product Types, sync checks).

## Usage

```bash
npx @vtech-com/capigo-skill@latest --dir ~/.openclaw/plugin-skills
```

Point `--dir` at whatever directory your agent runtime loads skills from. The skill is installed
at `<dir>/capigo-api`. The install is **idempotent** — any existing copy is removed first, so a
reference file dropped in a newer version can't linger as an orphan.

You can also set the directory via an environment variable:

```bash
CAPIGO_SKILLS_DIR=~/.openclaw/plugin-skills npx @vtech-com/capigo-skill@latest
```

The package version tracks the [`capigo` CLI release](https://github.com/vtech-com/capigo-api-sdk/releases);
pin it (`@0.9.0`) to install the skill matching a specific CLI version.

## What it installs

This package only *installs* the skill — you still need the `capigo` CLI itself
(`brew install vtech-com/tap/capigo` or see the [SDK repo](https://github.com/vtech-com/capigo-api-sdk)).
The skill teaches an agent how to use that CLI; it does not bundle it.

## License

[MIT](https://github.com/vtech-com/capigo-api-sdk/blob/main/LICENSE)
