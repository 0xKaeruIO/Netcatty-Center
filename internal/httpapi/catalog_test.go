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
	"netcatty-center/internal/security"
	"netcatty-center/internal/store"
)

func TestCatalogRequiresLiveAPIKey(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{
		Label:    "prod-web-1",
		Hostname: "10.0.1.12",
		Port:     22,
		Username: "deploy",
		Group:    "production/web",
		Tags:       []string{"linux", "prod"},
		Notes:      "入口机",
		Password:   "secret",
		PrivateKey: "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----",
	}); err != nil {
		t.Fatal(err)
	}
	generated := security.GenerateAPIKey()
	if _, err := st.CreateAPIKey("ci", generated.Hash, generated.Prefix, generated.Plaintext); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(st, config.Config{PublicDir: dir})

	denied := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	srv.Engine().ServeHTTP(denied, req)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", denied.Code)
	}

	ok := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Plaintext)
	srv.Engine().ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", ok.Code, ok.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(ok.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != float64(1) {
		t.Fatalf("version=%v", body["version"])
	}
	hosts, _ := body["hosts"].([]any)
	if len(hosts) != 1 {
		t.Fatalf("hosts=%v", body["hosts"])
	}
	host, _ := hosts[0].(map[string]any)
	if host["hostname"] != "10.0.1.12" {
		t.Fatalf("hostname=%v", host["hostname"])
	}
	if host["password"] != "secret" {
		t.Fatalf("password=%v", host["password"])
	}
	if host["privateKey"] != "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----" {
		t.Fatalf("privateKey=%v", host["privateKey"])
	}

	health := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/health", nil)
	srv.Engine().ServeHTTP(health, req)
	if health.Code != http.StatusOK {
		t.Fatalf("health=%d", health.Code)
	}
}

func TestAdminCanListStoredAPIKeyPlaintext(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	admin, err := st.CreateFirstAdmin("admin", "password123")
	if err != nil {
		t.Fatal(err)
	}
	generated := security.GenerateAPIKey()
	if _, err := st.CreateAPIKey("ops", generated.Hash, generated.Prefix, generated.Plaintext); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{PublicDir: dir}
	srv := New(st, cfg)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login=%d body=%s", rec.Code, rec.Body.String())
	}

	list := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/keys", nil)
	for _, cookie := range rec.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(list, req)
	if list.Code != http.StatusOK {
		t.Fatalf("list=%d body=%s", list.Code, list.Body.String())
	}
	var body struct {
		Keys []store.APIKey `json:"keys"`
	}
	if err := json.Unmarshal(list.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Keys) != 1 || body.Keys[0].Plaintext != generated.Plaintext {
		t.Fatalf("keys=%+v admin=%s", body.Keys, admin.ID)
	}
}

func TestCatalogIncludesStartupCommandRules(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	created, err := st.CreateHost(store.HostInput{
		Label:                 "jump",
		Hostname:              "10.0.1.1",
		Port:                  22,
		Username:              "root",
		StartupCommand:        "should-be-cleared",
		StartupCommandRunMode: "rules",
		StartupCommandRules: []store.StartupCommandRule{
			{Expect: "password:", Send: "secret"},
			{Expect: "", Send: "ssh deploy@10.0.1.12"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if created.StartupCommand != "" {
		t.Fatalf("rules mode should clear startupCommand, got %q", created.StartupCommand)
	}

	generated := security.GenerateAPIKey()
	if _, err := st.CreateAPIKey("ci", generated.Hash, generated.Prefix, generated.Plaintext); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(st, config.Config{PublicDir: dir})

	ok := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Plaintext)
	srv.Engine().ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", ok.Code, ok.Body.String())
	}
	var body struct {
		Hosts []store.CatalogHost `json:"hosts"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Hosts) != 1 {
		t.Fatalf("hosts=%v", body.Hosts)
	}
	host := body.Hosts[0]
	if host.StartupCommand != "" {
		t.Fatalf("catalog startupCommand=%q", host.StartupCommand)
	}
	if host.StartupCommandRunMode != "rules" {
		t.Fatalf("runMode=%q", host.StartupCommandRunMode)
	}
	if len(host.StartupCommandRules) != 2 {
		t.Fatalf("rules=%v", host.StartupCommandRules)
	}
	if host.StartupCommandRules[0].Expect != "password:" || host.StartupCommandRules[0].Send != "secret" {
		t.Fatalf("rule0=%+v", host.StartupCommandRules[0])
	}
	if host.StartupCommandRules[1].Expect != "" || host.StartupCommandRules[1].Send != "ssh deploy@10.0.1.12" {
		t.Fatalf("rule1=%+v", host.StartupCommandRules[1])
	}
}

func TestAdminHostStartupCommandRoundTrip(t *testing.T) {
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

	create := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/hosts", strings.NewReader(`{
		"label":"web-1",
		"hostname":"10.0.1.12",
		"port":22,
		"username":"deploy",
		"startupCommand":"tmux attach || tmux",
		"startupCommandRunMode":"lineDelay",
		"startupCommandRules":[{"expect":"ignored","send":"kept-for-later"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(create, req)
	if create.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", create.Code, create.Body.String())
	}
	var created struct {
		Host store.Host `json:"host"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Host.StartupCommand != "tmux attach || tmux" {
		t.Fatalf("startupCommand=%q", created.Host.StartupCommand)
	}
	if created.Host.StartupCommandRunMode != "lineDelay" {
		t.Fatalf("runMode=%q", created.Host.StartupCommandRunMode)
	}

	update := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPut, "/api/admin/hosts/"+created.Host.ID, strings.NewReader(`{
		"label":"web-1",
		"hostname":"10.0.1.12",
		"startupCommandRunMode":"bogus",
		"startupCommandRules":[]
	}`))
	req.Header.Set("Content-Type", "application/json")
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(update, req)
	if update.Code != http.StatusOK {
		t.Fatalf("update=%d body=%s", update.Code, update.Body.String())
	}
	var updated struct {
		Host store.Host `json:"host"`
	}
	if err := json.Unmarshal(update.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.Host.StartupCommandRunMode != "" {
		t.Fatalf("invalid runMode should normalize empty, got %q", updated.Host.StartupCommandRunMode)
	}
	if updated.Host.StartupCommandRules == nil {
		t.Fatal("rules should be empty slice, not null")
	}
}

func testPublicDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBootstrapAdminLoginDoesNotUseDatabasePassword(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	srv := New(st, config.Config{
		PublicDir:     testPublicDir(t),
		AdminUser:     "ops",
		AdminPassword: "flag-password",
	})

	status := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/setup-status", nil)
	srv.Engine().ServeHTTP(status, req)
	if status.Code != http.StatusOK {
		t.Fatalf("status=%d", status.Code)
	}
	var setup map[string]any
	if err := json.Unmarshal(status.Body.Bytes(), &setup); err != nil {
		t.Fatal(err)
	}
	if setup["needsSetup"] != false {
		t.Fatalf("needsSetup=%v", setup["needsSetup"])
	}

	denied := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"ops","password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(denied, req)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("denied=%d", denied.Code)
	}

	ok := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"OPS","password":"flag-password"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("login=%d body=%s", ok.Code, ok.Body.String())
	}
	var body struct {
		Admin store.Admin `json:"admin"`
	}
	if err := json.Unmarshal(ok.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Admin.Username != "ops" {
		t.Fatalf("admin=%+v", body.Admin)
	}

	me := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/admin/me", nil)
	for _, cookie := range ok.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(me, req)
	if me.Code != http.StatusOK {
		t.Fatalf("me=%d body=%s", me.Code, me.Body.String())
	}
}

func TestRevokedAPIKeyCanBeRestoredOrDeleted(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{Label: "web", Hostname: "10.0.1.12"}); err != nil {
		t.Fatal(err)
	}
	generated := security.GenerateAPIKey()
	key, err := st.CreateAPIKey("ops", generated.Hash, generated.Prefix, generated.Plaintext)
	if err != nil {
		t.Fatal(err)
	}

	srv := New(st, config.Config{PublicDir: testPublicDir(t)})
	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login=%d", login.Code)
	}

	revoke := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/admin/keys/"+key.ID, nil)
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(revoke, req)
	if revoke.Code != http.StatusOK {
		t.Fatalf("revoke=%d body=%s", revoke.Code, revoke.Body.String())
	}

	denied := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Plaintext)
	srv.Engine().ServeHTTP(denied, req)
	if denied.Code != http.StatusUnauthorized {
		t.Fatalf("revoked catalog=%d", denied.Code)
	}

	restore := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/keys/"+key.ID+"/restore", nil)
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(restore, req)
	if restore.Code != http.StatusOK {
		t.Fatalf("restore=%d body=%s", restore.Code, restore.Body.String())
	}

	ok := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Plaintext)
	srv.Engine().ServeHTTP(ok, req)
	if ok.Code != http.StatusOK {
		t.Fatalf("restored catalog=%d", ok.Code)
	}

	del := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/keys/"+key.ID+"/delete", nil)
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
	srv.Engine().ServeHTTP(del, req)
	if del.Code != http.StatusOK {
		t.Fatalf("delete=%d body=%s", del.Code, del.Body.String())
	}

	gone := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+generated.Plaintext)
	srv.Engine().ServeHTTP(gone, req)
	if gone.Code != http.StatusUnauthorized {
		t.Fatalf("deleted catalog=%d", gone.Code)
	}
}

func TestCatalogFiltersHostsByVisibleKeys(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}

	alpha := security.GenerateAPIKey()
	beta := security.GenerateAPIKey()
	keyA, err := st.CreateAPIKey("alpha", alpha.Hash, alpha.Prefix, alpha.Plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateAPIKey("beta", beta.Hash, beta.Prefix, beta.Plaintext); err != nil {
		t.Fatal(err)
	}

	if _, err := st.CreateHost(store.HostInput{
		Label:    "shared",
		Hostname: "10.0.1.10",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{
		Label:         "secret",
		Hostname:      "10.0.1.11",
		Visibility:    "keys",
		VisibleKeyIDs: []string{keyA.ID},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{
		Label:         "hidden",
		Hostname:      "10.0.1.12",
		Visibility:    "keys",
		VisibleKeyIDs: []string{},
	}); err != nil {
		t.Fatal(err)
	}

	srv := New(st, config.Config{PublicDir: testPublicDir(t)})
	catalogHostnames := func(plaintext string) []string {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
		req.Header.Set("Authorization", "Bearer "+plaintext)
		srv.Engine().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("catalog=%d body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Hosts []store.CatalogHost `json:"hosts"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(body.Hosts))
		for _, host := range body.Hosts {
			names = append(names, host.Label)
		}
		return names
	}

	alphaHosts := catalogHostnames(alpha.Plaintext)
	if strings.Join(alphaHosts, ",") != "secret,shared" {
		t.Fatalf("alpha hosts=%v", alphaHosts)
	}
	betaHosts := catalogHostnames(beta.Plaintext)
	if strings.Join(betaHosts, ",") != "shared" {
		t.Fatalf("beta hosts=%v", betaHosts)
	}
}
