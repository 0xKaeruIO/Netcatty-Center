package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"netcatty-center/internal/config"
	"netcatty-center/internal/security"
	"netcatty-center/internal/store"
)

func TestAPIKeyDefaultsToReadOnlyAndShareStillWorks(t *testing.T) {
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

	srv := New(st, config.Config{})
	login := adminLogin(t, srv)

	created := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/keys", strings.NewReader(`{"name":"ops"}`))
	req.Header.Set("Content-Type", "application/json")
	copyCookies(req, login)
	srv.Engine().ServeHTTP(created, req)
	if created.Code != http.StatusCreated {
		t.Fatalf("create key=%d body=%s", created.Code, created.Body.String())
	}
	var issued struct {
		Key store.APIKey `json:"key"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if issued.Key.Permission != store.KeyPermissionRead {
		t.Fatalf("default permission=%q", issued.Key.Permission)
	}
	plaintext := issued.Key.Plaintext

	catalog := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	srv.Engine().ServeHTTP(catalog, req)
	if catalog.Code != http.StatusOK {
		t.Fatalf("catalog=%d body=%s", catalog.Code, catalog.Body.String())
	}
	var catalogBody map[string]any
	if err := json.Unmarshal(catalog.Body.Bytes(), &catalogBody); err != nil {
		t.Fatal(err)
	}
	if catalogBody["permission"] != "read" {
		t.Fatalf("catalog permission=%v", catalogBody["permission"])
	}

	denied := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/hosts", strings.NewReader(`{"label":"new","hostname":"10.0.0.9"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+plaintext)
	srv.Engine().ServeHTTP(denied, req)
	if denied.Code != http.StatusForbidden {
		t.Fatalf("read-only host write=%d body=%s", denied.Code, denied.Body.String())
	}

	share := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/share/rooms", strings.NewReader(`{"label":"web"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+plaintext)
	srv.Engine().ServeHTTP(share, req)
	if share.Code != http.StatusCreated {
		t.Fatalf("read-only share create=%d body=%s", share.Code, share.Body.String())
	}
	var room map[string]any
	if err := json.Unmarshal(share.Body.Bytes(), &room); err != nil {
		t.Fatal(err)
	}
	pin, _ := room["pin"].(string)
	roomID, _ := room["roomId"].(string)

	joined := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/share/join", strings.NewReader(`{"pin":"`+pin+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+plaintext)
	srv.Engine().ServeHTTP(joined, req)
	if joined.Code != http.StatusOK {
		t.Fatalf("read-only share join=%d body=%s", joined.Code, joined.Body.String())
	}

	closed := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodDelete, "/api/v1/share/rooms/"+roomID, nil)
	req.Header.Set("Authorization", "Bearer "+plaintext)
	srv.Engine().ServeHTTP(closed, req)
	if closed.Code != http.StatusOK {
		t.Fatalf("read-only share close=%d body=%s", closed.Code, closed.Body.String())
	}
}

func TestReadWriteKeyCanMutateHostList(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}

	srv := New(st, config.Config{})
	login := adminLogin(t, srv)

	created := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/keys", strings.NewReader(`{"name":"writer","permission":"readwrite"}`))
	req.Header.Set("Content-Type", "application/json")
	copyCookies(req, login)
	srv.Engine().ServeHTTP(created, req)
	if created.Code != http.StatusCreated {
		t.Fatalf("create key=%d body=%s", created.Code, created.Body.String())
	}
	var issued struct {
		Key store.APIKey `json:"key"`
	}
	if err := json.Unmarshal(created.Body.Bytes(), &issued); err != nil {
		t.Fatal(err)
	}
	if issued.Key.Permission != store.KeyPermissionReadWrite {
		t.Fatalf("permission=%q", issued.Key.Permission)
	}

	add := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/v1/hosts", strings.NewReader(`{"label":"db-1","hostname":"10.0.2.8","group":"production/db"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+issued.Key.Plaintext)
	srv.Engine().ServeHTTP(add, req)
	if add.Code != http.StatusCreated {
		t.Fatalf("write host=%d body=%s", add.Code, add.Body.String())
	}

	hosts, err := st.ListHosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Label != "db-1" || hosts[0].Group != "production/db" {
		t.Fatalf("hosts=%+v", hosts)
	}
}

func TestAdminCanChangeKeyPermission(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	generated := security.GenerateAPIKey()
	key, err := st.CreateAPIKey("ops", generated.Hash, generated.Prefix, generated.Plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if key.Permission != store.KeyPermissionRead {
		t.Fatalf("permission=%q", key.Permission)
	}

	srv := New(st, config.Config{})
	login := adminLogin(t, srv)

	updated := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPut, "/api/admin/keys/"+key.ID, strings.NewReader(`{"permission":"readwrite"}`))
	req.Header.Set("Content-Type", "application/json")
	copyCookies(req, login)
	srv.Engine().ServeHTTP(updated, req)
	if updated.Code != http.StatusOK {
		t.Fatalf("update=%d body=%s", updated.Code, updated.Body.String())
	}
	var body struct {
		Key store.APIKey `json:"key"`
	}
	if err := json.Unmarshal(updated.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Key.Permission != store.KeyPermissionReadWrite {
		t.Fatalf("permission=%q", body.Key.Permission)
	}
}

func adminLogin(t *testing.T, srv *Server) *httptest.ResponseRecorder {
	t.Helper()
	login := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/login", strings.NewReader(`{"username":"admin","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Engine().ServeHTTP(login, req)
	if login.Code != http.StatusOK {
		t.Fatalf("login=%d body=%s", login.Code, login.Body.String())
	}
	return login
}

func copyCookies(req *http.Request, login *httptest.ResponseRecorder) {
	for _, cookie := range login.Result().Cookies() {
		req.AddCookie(cookie)
	}
}
