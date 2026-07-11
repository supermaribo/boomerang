package api

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boomerang-backup/boomerang/internal/config"
	"github.com/boomerang-backup/boomerang/internal/store"
	"testing/fstest"
)

func TestSpaHandlerServesIndexForAppRoute(t *testing.T) {
	webFS := fstest.MapFS{
		"index.html":              {Data: []byte(`<html><body><div id="root"></div><script src="/assets/app.js"></script></body></html>`)},
		"assets/app.js":           {Data: []byte("console.log('ok')")},
	}
	st, err := store.Open(t.TempDir() + "/app.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(&config.Config{}, st, nil, webFS, nil)

	req := httptest.NewRequest(http.MethodGet, "/app", nil)
	rec := httptest.NewRecorder()
	srv.spaHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if !contains(string(body), `id="root"`) {
		t.Fatalf("expected index.html body, got %q", body)
	}
}

func TestSpaHandlerServesAsset(t *testing.T) {
	webFS := fstest.MapFS{
		"index.html":    {Data: []byte(`<html></html>`)},
		"assets/app.js": {Data: []byte("console.log('ok')")},
	}
	st, err := store.Open(t.TempDir() + "/app.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(&config.Config{}, st, nil, webFS, nil)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	srv.spaHandler().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body, _ := io.ReadAll(rec.Body)
	if string(body) != "console.log('ok')" {
		t.Fatalf("asset body = %q", body)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
