package httpapi

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"netcatty-center/internal/config"
	"netcatty-center/internal/store"
)

func TestEmbeddedPublicUIIsServed(t *testing.T) {
	st, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	srv := New(st, config.Config{})

	index := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	srv.Engine().ServeHTTP(index, req)
	if index.Code != http.StatusOK {
		t.Fatalf("index=%d", index.Code)
	}
	body := index.Body.String()
	if !strings.Contains(body, "Netcatty Center") {
		t.Fatalf("index body=%q", body)
	}
	if !strings.Contains(body, "/assets/") {
		t.Fatalf("index missing hashed assets: %q", body)
	}

	asset := regexp.MustCompile(`/assets/[^"']+\.js`).FindString(body)
	if asset == "" {
		t.Fatalf("no js asset in index: %q", body)
	}
	js := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, asset, nil)
	srv.Engine().ServeHTTP(js, req)
	if js.Code != http.StatusOK {
		t.Fatalf("%s=%d", asset, js.Code)
	}
	if !strings.Contains(js.Body.String(), "Netcatty") && js.Body.Len() < 100 {
		t.Fatalf("%s too small: %d", asset, js.Body.Len())
	}

	spa := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/hosts", nil)
	srv.Engine().ServeHTTP(spa, req)
	if spa.Code != http.StatusOK || !strings.Contains(spa.Body.String(), "Netcatty Center") {
		t.Fatalf("spa fallback=%d", spa.Code)
	}
}
