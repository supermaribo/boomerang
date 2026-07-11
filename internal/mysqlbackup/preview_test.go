package mysqlbackup

import (
	"encoding/json"
	"testing"
)

func TestRestorePreviewResultJSON(t *testing.T) {
	res := normalizePreviewResult(RestorePreviewResult{Message: "ok"})
	b, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"tables", "onlyBackup", "onlyLive"} {
		v, ok := m[key]
		if !ok {
			t.Fatalf("missing %q in JSON", key)
		}
		if v == nil {
			t.Fatalf("%q must not encode as null (breaks restore UI)", key)
		}
		if _, ok := v.([]any); !ok {
			t.Fatalf("%q must be a JSON array", key)
		}
	}
}
