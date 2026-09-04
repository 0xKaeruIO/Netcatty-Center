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

func setupShareServer(t *testing.T) (*Server, string) {
	t.Helper()
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
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
	return New(st, config.Config{PublicDir: dir}), generated.Plaintext
}

func TestShareRoomRequiresAPIKey(t *testing.T) {
	srv, _ := setupShareServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/share/rooms", strings.NewReader(`{"label":"web"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestShareCreateJoinAndCatalogUntouched(t *testing.T) {
	srv, key := setupShareServer(t)

	created := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/share/rooms", strings.NewReader(`{"label":"web","cols":120,"rows":40}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	srv.Engine().ServeHTTP(created, req)
	if created.Code != http.StatusCreated {
		t.Fatalf("create=%d body=%s", created.Code, created.Body.String())
	}
	var room map[string]any
	if err := json.Unmarshal(created.Body.Bytes(), &room); err != nil {
		t.Fatal(err)
	}
	pin, _ := room["pin"].(string)
	if len(pin) != 6 {
		t.Fatalf("pin=%v", room["pin"])
	}

	joined := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/share/join", strings.NewReader(`{"pin":"`+pin+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	srv.Engine().ServeHTTP(joined, req)
	if joined.Code != http.StatusOK {
		t.Fatalf("join=%d body=%s", joined.Code, joined.Body.String())
	}

	catalog := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+key)
	srv.Engine().ServeHTTP(catalog, req)
	if catalog.Code != http.StatusOK {
		t.Fatalf("catalog=%d", catalog.Code)
	}

	wrong := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/share/join", strings.NewReader(`{"pin":"000000"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)
	srv.Engine().ServeHTTP(wrong, req)
	if wrong.Code != http.StatusUnauthorized {
		t.Fatalf("wrong pin=%d", wrong.Code)
	}
}
