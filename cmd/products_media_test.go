package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProductsMediaAddFromURLSendsJSON(t *testing.T) {
	mediaAddTenant, mediaAddSourceURL, mediaAddFilename = "acme", "https://x.example/a.jpg", " front.jpg "
	mediaAddVariantIDs, mediaAddDefault, mediaAddIdempotencyKey = []string{"v1"}, true, "k1"
	t.Cleanup(func() {
		mediaAddTenant, mediaAddSourceURL, mediaAddFilename = "", "", ""
		mediaAddVariantIDs, mediaAddDefault, mediaAddIdempotencyKey = nil, false, ""
	})

	seen, printed := callAgainst(t, `{"data":{"id":"m1"}}`,
		func() { _ = productsMediaAddCmd.RunE(productsMediaAddCmd, []string{"p1"}) })

	if seen.method != "POST" || seen.path != "/pcms/products/p1/media" || seen.tenant != "acme" {
		t.Errorf("call = %s %s tenant=%q", seen.method, seen.path, seen.tenant)
	}
	if seen.body["source_url"] != "https://x.example/a.jpg" || seen.body["filename"] != "front.jpg" || seen.body["is_default"] != true {
		t.Errorf("body = %v", seen.body)
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}

func TestProductsMediaAddFileSendsMultipartWithFields(t *testing.T) {
	path := filepath.Join(t.TempDir(), "red.jpg")
	if err := os.WriteFile(path, []byte("\xff\xd8\xff\xe0jpegbytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	mediaAddTenant, mediaAddFile = "acme", path
	mediaAddVariantIDs, mediaAddDefault = []string{"v1", "v2"}, true
	t.Cleanup(func() {
		mediaAddTenant, mediaAddFile, mediaAddVariantIDs, mediaAddDefault = "", "", nil, false
	})

	seen, _ := callAgainst(t, `{"data":{"id":"m1"}}`,
		func() { _ = productsMediaAddCmd.RunE(productsMediaAddCmd, []string{"p1"}) })

	if seen.path != "/pcms/products/p1/media" {
		t.Errorf("path = %s", seen.path)
	}
	for _, want := range []string{`name="file"; filename="red.jpg"`, `name="variant_ids"`, "v1", "v2", `name="is_default"`, "true"} {
		if !strings.Contains(seen.rawBody, want) {
			t.Errorf("multipart body lacks %q", want)
		}
	}
}

func TestProductsMediaUpdateSendsOnlyTheFlagsGiven(t *testing.T) {
	mediaUpdateTenant, mediaUpdatePosition, mediaUpdateClearLinks, mediaUpdateClearAltText = "acme", 1, true, true
	_ = productsMediaUpdateCmd.Flags().Set("position", "1")
	t.Cleanup(func() {
		mediaUpdateTenant, mediaUpdatePosition, mediaUpdateClearLinks, mediaUpdateClearAltText = "", 0, false, false
		productsMediaUpdateCmd.Flags().Lookup("position").Changed = false
	})

	seen, _ := callAgainst(t, `{"data":{"id":"m1"}}`,
		func() { _ = productsMediaUpdateCmd.RunE(productsMediaUpdateCmd, []string{"p1", "m1"}) })

	if seen.method != "PATCH" || seen.path != "/pcms/products/p1/media/m1" {
		t.Errorf("call = %s %s", seen.method, seen.path)
	}
	if got := keysOf(seen.body); !sameKeys(got, []string{"alt_text", "position", "variant_ids"}) {
		t.Errorf("body keys = %v", got)
	}
	if v, ok := seen.body["alt_text"]; !ok || v != nil {
		t.Errorf("alt_text = %v, want null from --clear-alt-text", v)
	}
	if links, _ := seen.body["variant_ids"].([]any); len(links) != 0 {
		t.Errorf("variant_ids = %v, want an empty array from --clear-variants", seen.body["variant_ids"])
	}
}

func TestProductsMediaDeleteCallsDelete(t *testing.T) {
	mediaDeleteTenant = "acme"
	t.Cleanup(func() { mediaDeleteTenant = "" })

	seen, printed := callAgainst(t, `{"data":{"deleted":true}}`,
		func() { _ = productsMediaDeleteCmd.RunE(productsMediaDeleteCmd, []string{"p1", "m1"}) })

	if seen.method != "DELETE" || seen.path != "/pcms/products/p1/media/m1" || seen.hasBody {
		t.Errorf("call = %s %s hasBody=%v", seen.method, seen.path, seen.hasBody)
	}
	if printed == "" {
		t.Error("nothing printed")
	}
}
