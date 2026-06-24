package version

import (
	"strings"
	"testing"
)

func TestString_ContainsVersion(t *testing.T) {
	s := String()
	if !strings.Contains(s, "ztutor") {
		t.Errorf("String() = %q, should contain ztutor", s)
	}
	if !strings.Contains(s, Version) {
		t.Errorf("String() = %q, should contain version %q", s, Version)
	}
}

func TestString_NotEmpty(t *testing.T) {
	s := String()
	if s == "" {
		t.Error("String() should not be empty")
	}
}

func TestDefaults(t *testing.T) {
	if Version == "" {
		t.Error("Version should not be empty")
	}
	if Commit == "" {
		t.Error("Commit should not be empty")
	}
	if BuildDate == "" {
		t.Error("BuildDate should not be empty")
	}
}
