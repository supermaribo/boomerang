package update

import "testing"

func TestIsNewer(t *testing.T) {
	if !isNewer("0.1.0", "0.2.0") {
		t.Fatal("expected 0.2.0 newer than 0.1.0")
	}
	if isNewer("0.2.0", "0.2.0") {
		t.Fatal("same version should not be newer")
	}
	if !isNewer("dev", "1.0.0") {
		t.Fatal("dev build should accept release update")
	}
}

func TestDisplayVersion(t *testing.T) {
	if displayVersion("v1.2.3") != "1.2.3" {
		t.Fatal("expected strip v prefix")
	}
}
