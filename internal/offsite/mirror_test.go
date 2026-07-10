package offsite

import "testing"

func TestShouldSkip(t *testing.T) {
	if !shouldSkip(".hidden") {
		t.Fatal("expected skip hidden")
	}
	if shouldSkip("backups/files/x") {
		t.Fatal("expected keep backup path")
	}
	if shouldSkip("secrets/master.key") {
		t.Fatal("expected keep master key")
	}
}

func TestObjectPrefix(t *testing.T) {
	cfg := Config{Prefix: ""}
	if cfg.ObjectPrefix() != "boomerang" {
		t.Fatalf("got %q", cfg.ObjectPrefix())
	}
	cfg.Prefix = "my-appliance/"
	if cfg.ObjectPrefix() != "my-appliance" {
		t.Fatalf("got %q", cfg.ObjectPrefix())
	}
}
