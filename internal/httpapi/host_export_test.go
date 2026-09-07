package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"netcatty-center/internal/config"
	"netcatty-center/internal/store"
)

func TestExportHostsMatchesImportJSON(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(st, config.Config{PublicDir: dir})
	login := adminLogin(t, srv)

	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "hosts-import.json"))
	if err != nil {
		t.Fatal(err)
	}
	imported := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/hosts/import", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	copyCookies(req, login)
	srv.Engine().ServeHTTP(imported, req)
	if imported.Code != http.StatusCreated {
		t.Fatalf("import=%d body=%s", imported.Code, imported.Body.String())
	}

	rec := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/hosts/export", nil)
	copyCookies(req, login)
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("export=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Header().Get("Content-Disposition"), "hosts-export.json") {
		t.Fatalf("disposition=%s", rec.Header().Get("Content-Disposition"))
	}
	if !strings.Contains(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("content-type=%s", rec.Header().Get("Content-Type"))
	}

	groups, inputs, err := parseHostImportJSON(rec.Body.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(groups, ",") != "production,production/web,production/jump,network,staging,staging/empty" {
		t.Fatalf("groups=%v", groups)
	}
	if len(inputs) != 3 {
		t.Fatalf("hosts=%d", len(inputs))
	}

	byLabel := map[string]store.HostInput{}
	for _, host := range inputs {
		byLabel[host.Label] = host
	}
	web := byLabel["prod-web-1"]
	if web.Hostname != "10.0.1.12" || web.Password != "change-me" || web.StartupCommand != "tmux attach || tmux new -s ops" {
		t.Fatalf("web=%+v", web)
	}
	jump := byLabel["prod-jump"]
	if jump.Password != "jump-secret" || jump.Passphrase != "key-passphrase" || jump.PrivateKey == "" {
		t.Fatalf("jump secrets=%+v", jump)
	}
	if jump.StartupCommandRunMode != "rules" || jump.StartupCommand != "" || len(jump.StartupCommandRules) != 2 {
		t.Fatalf("jump rules=%+v", jump)
	}
	sw := byLabel["core-sw-1"]
	if sw.Protocol != "telnet" || sw.Port != 23 || sw.StartupCommandRunMode != "lineDelay" {
		t.Fatalf("switch=%+v", sw)
	}

	var file hostExportFile
	if err := json.Unmarshal(rec.Body.Bytes(), &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Hosts) != 3 {
		t.Fatalf("file hosts=%d", len(file.Hosts))
	}
	text := rec.Body.String()
	if strings.Contains(text, `"id"`) || strings.Contains(text, `"createdAt"`) || strings.Contains(text, `"updatedAt"`) {
		t.Fatalf("export should match import JSON without catalog ids, got %s", text)
	}
}

func TestExportHostsRequiresAdmin(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(st, config.Config{PublicDir: dir})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/hosts/export", nil)
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExportEmptyCatalogIsReimportable(t *testing.T) {
	raw, err := marshalHostExportJSON(nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	groups, inputs, err := parseHostImportJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 || len(inputs) != 0 {
		t.Fatalf("groups=%v inputs=%d", groups, len(inputs))
	}
}
