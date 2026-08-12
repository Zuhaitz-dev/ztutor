package apputil

import (
	"path/filepath"
	"testing"
)

func TestDefaultDataDir_UsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/ztutor-xdg")
	got := DefaultDataDir()
	want := filepath.Join("/tmp/ztutor-xdg", "ztutor")
	if got != want {
		t.Fatalf("DefaultDataDir = %q, want %q", got, want)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("ZTUTOR_TEST_ENV", "from-env")
	if got := EnvOrDefault("ZTUTOR_TEST_ENV", "fallback"); got != "from-env" {
		t.Fatalf("EnvOrDefault with env = %q, want from-env", got)
	}
	if got := EnvOrDefault("ZTUTOR_MISSING_ENV", "fallback"); got != "fallback" {
		t.Fatalf("EnvOrDefault fallback = %q, want fallback", got)
	}
}
