package httpapi

import (
	"net/http"
	"net/http/httptest"
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
	if !strings.Contains(index.Body.String(), "Netcatty Center") {
		t.Fatalf("index body=%q", index.Body.String())
	}

	js := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/app.js", nil)
	srv.Engine().ServeHTTP(js, req)
	if js.Code != http.StatusOK || !strings.Contains(js.Body.String(), "function boot") {
		t.Fatalf("app.js=%d body=%q", js.Code, js.Body.String()[:min(80, js.Body.Len())])
	}

	css := httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/styles.css", nil)
	srv.Engine().ServeHTTP(css, req)
	if css.Code != http.StatusOK || !strings.Contains(css.Body.String(), "--brass") {
		t.Fatalf("styles.css=%d", css.Code)
	}
}
