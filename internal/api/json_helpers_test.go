package api

import (
	"encoding/json"
	"testing"
)

func TestNormalizeNilSlices(t *testing.T) {
	var nilTargets []targetHealthRow
	out := normalizeNilSlices(map[string]any{"targets": nilTargets})
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", out)
	}
	targets, ok := m["targets"].([]targetHealthRow)
	if !ok {
		t.Fatalf("expected []targetHealthRow, got %T", m["targets"])
	}
	if targets == nil || len(targets) != 0 {
		t.Fatalf("expected empty non-nil slice, got %#v", targets)
	}

	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"targets":[]}` {
		t.Fatalf("json = %s", b)
	}
}

// Regression: a struct with a nil pointer field must not panic
// (reflect.Value.Set on zero Value) and must keep the field as JSON null.
func TestNormalizeNilSlicesNilPointerField(t *testing.T) {
	dto := databaseDTO{ID: "x", Name: "db", FileServerID: nil, IncludeTables: nil}
	out := normalizeNilSlices(dto)
	got, ok := out.(databaseDTO)
	if !ok {
		t.Fatalf("expected databaseDTO, got %T", out)
	}
	if got.FileServerID != nil {
		t.Fatalf("expected nil FileServerID, got %v", *got.FileServerID)
	}
	if got.IncludeTables == nil || len(got.IncludeTables) != 0 {
		t.Fatalf("expected empty non-nil IncludeTables, got %#v", got.IncludeTables)
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["fileServerId"] != nil {
		t.Fatalf("fileServerId = %v, want null", m["fileServerId"])
	}

	// Non-nil pointer survives round-trip.
	fs := "abc"
	out2 := normalizeNilSlices(databaseDTO{FileServerID: &fs})
	if got2 := out2.(databaseDTO); got2.FileServerID == nil || *got2.FileServerID != "abc" {
		t.Fatalf("pointer field lost: %#v", got2.FileServerID)
	}
}
