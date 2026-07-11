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
