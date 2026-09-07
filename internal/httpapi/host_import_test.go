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

func TestParseHostImportJSONShapes(t *testing.T) {
	groups, inputs, err := parseHostImportJSON([]byte(`[{"label":"a","hostname":"10.0.0.1"}]`))
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 0 || len(inputs) != 1 || inputs[0].Label != "a" {
		t.Fatalf("array import groups=%v inputs=%+v", groups, inputs)
	}

	groups, inputs, err = parseHostImportJSON([]byte(`{
		"groups":["staging/empty"],
		"hosts":[{"label":"b","hostname":"10.0.0.2","startupCommandRunMode":"rules","startupCommandRules":[{"expect":"p:","send":"x"}]}]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(groups, ",") != "staging/empty" {
		t.Fatalf("groups=%v", groups)
	}
	if len(inputs) != 1 || inputs[0].StartupCommandRunMode != "rules" || inputs[0].StartupCommandRules[0].Send != "x" {
		t.Fatalf("object import=%+v", inputs)
	}

	groups, inputs, err = parseHostImportJSON([]byte(`{"label":"solo","hostname":"10.0.0.3","group":"lab"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(inputs) != 1 || inputs[0].Group != "lab" || len(groups) != 0 {
		t.Fatalf("single host=%+v groups=%v", inputs, groups)
	}

	if _, _, err := parseHostImportJSON([]byte("")); err == nil {
		t.Fatal("expected empty error")
	}
}

func TestImportHostsFromExampleJSON(t *testing.T) {
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

	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login=%d body=%s", login.Code, login.Body.String())
	}

	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "hosts-import.json"))
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/hosts/import", strings.NewReader(string(raw)))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("import=%d body=%s", rec.Code, rec.Body.String())
	}

	var body struct {
		Imported int          `json:"imported"`
		Groups   []string     `json:"groups"`
		Hosts    []store.Host `json:"hosts"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Imported != 3 || len(body.Hosts) != 3 {
		t.Fatalf("imported=%d hosts=%d", body.Imported, len(body.Hosts))
	}
	if strings.Join(body.Groups, ",") != "production,production/web,production/jump,network,staging,staging/empty" {
		t.Fatalf("groups=%v", body.Groups)
	}

	byLabel := map[string]store.Host{}
	for _, host := range body.Hosts {
		byLabel[host.Label] = host
	}
	web := byLabel["prod-web-1"]
	if web.Hostname != "10.0.1.12" || web.Password != "change-me" || web.StartupCommand != "tmux attach || tmux new -s ops" || web.StartupCommandRunMode != "paste" {
		t.Fatalf("web=%+v", web)
	}
	jump := byLabel["prod-jump"]
	if jump.Group != "production/jump" || jump.StartupCommandRunMode != "rules" || jump.StartupCommand != "" {
		t.Fatalf("jump=%+v", jump)
	}
	if len(jump.StartupCommandRules) != 2 || jump.StartupCommandRules[0].Expect != "password:" || jump.StartupCommandRules[1].Send != "ssh deploy@10.0.1.12" {
		t.Fatalf("jump rules=%+v", jump.StartupCommandRules)
	}
	if jump.PrivateKey == "" || jump.Passphrase != "key-passphrase" {
		t.Fatalf("jump secrets=%+v", jump)
	}
	sw := byLabel["core-sw-1"]
	if sw.Protocol != "telnet" || sw.DeviceType != "network" || sw.Port != 23 || sw.StartupCommandRunMode != "lineDelay" {
		t.Fatalf("switch=%+v", sw)
	}
}

func TestImportHostsRejectsInvalidHost(t *testing.T) {
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

	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(login, req)

	rec := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/hosts/import", strings.NewReader(`{"hosts":[{"label":"broken"}]}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "主机名") {
		t.Fatalf("body=%s", rec.Body.String())
	}
	hosts, err := st.ListHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 0 {
		t.Fatalf("partial import hosts=%d", len(hosts))
	}
}
