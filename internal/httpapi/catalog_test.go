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
