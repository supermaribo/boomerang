package tzutil

import "testing"

func TestNormalize(t *testing.T) {
	got, err := Normalize("Europe/London")
	if err != nil || got != "Europe/London" {
		t.Fatalf("got %q err %v", got, err)
	}
	if _, err := Normalize("Not/A/Zone"); err == nil {
		t.Fatal("expected error")
	}
	if got, err := Normalize(""); err != nil || got != "UTC" {
		t.Fatalf("empty -> UTC, got %q err %v", got, err)
	}
}
