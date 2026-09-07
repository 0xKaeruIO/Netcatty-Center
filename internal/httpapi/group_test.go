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

func TestDeleteGroupMovesHostsAndDeleteGroupWithHostsRemovesThem(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if _, err := st.CreateFirstAdmin("admin", "password123"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{Label: "web-1", Hostname: "10.0.1.12", Group: "production/web"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{Label: "db-1", Hostname: "10.0.1.20", Group: "production/db"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateHost(store.HostInput{Label: "jump", Hostname: "10.0.1.5", Group: "ops"}); err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<html></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	srv := New(st, config.Config{PublicDir: dir})
	login := adminLogin(t, srv)

	moved := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/admin/groups/delete", strings.NewReader(`{"path":"production/web"}`))
	req.Header.Set("Content-Type", "application/json")
	copyCookies(req, login)
	srv.Engine().ServeHTTP(moved, req)
	if moved.Code != http.StatusOK {
		t.Fatalf("delete group=%d body=%s", moved.Code, moved.Body.String())
	}
	var afterMove struct {
		Groups []string     `json:"groups"`
		Hosts  []store.Host `json:"hosts"`
	}
	if err := json.Unmarshal(moved.Body.Bytes(), &afterMove); err != nil {
		t.Fatal(err)
	}
	byLabel := map[string]store.Host{}
	for _, host := range afterMove.Hosts {
		byLabel[host.Label] = host
	}
	if byLabel["web-1"].Group != "production" {
		t.Fatalf("web-1 group=%q", byLabel["web-1"].Group)
	}
	if byLabel["db-1"].Group != "production/db" {
		t.Fatalf("db-1 group=%q", byLabel["db-1"].Group)
	}

	removed := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodPost, "/api/admin/groups/delete", strings.NewReader(`{"path":"production","deleteHosts":true}`))
	req.Header.Set("Content-Type", "application/json")
	copyCookies(req, login)
	srv.Engine().ServeHTTP(removed, req)
	if removed.Code != http.StatusOK {
		t.Fatalf("delete group and hosts=%d body=%s", removed.Code, removed.Body.String())
	}
	var afterDelete struct {
		Groups []string     `json:"groups"`
		Hosts  []store.Host `json:"hosts"`
	}
	if err := json.Unmarshal(removed.Body.Bytes(), &afterDelete); err != nil {
		t.Fatal(err)
	}
	if len(afterDelete.Hosts) != 1 || afterDelete.Hosts[0].Label != "jump" {
		t.Fatalf("hosts=%+v", afterDelete.Hosts)
	}
	for _, group := range afterDelete.Groups {
		if group == "production" || strings.HasPrefix(group, "production/") {
			t.Fatalf("leftover group=%q groups=%v", group, afterDelete.Groups)
		}
	}
}
