package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/boomerang-backup/boomerang/internal/config"
	"github.com/boomerang-backup/boomerang/internal/store"
)

func TestTargetHealthReturnsEmptyArray(t *testing.T) {
	st, err := store.Open(t.TempDir() + "/app.db")
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	srv := New(&config.Config{}, st, nil, nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/target-health", nil)
	rec := httptest.NewRecorder()
	srv.handleTargetHealth(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var body struct {
		Targets []targetHealthRow `json:"targets"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Targets == nil {
		t.Fatal("targets was null, want []")
	}
	if len(body.Targets) != 0 {
		t.Fatalf("targets len = %d", len(body.Targets))
	}
}
