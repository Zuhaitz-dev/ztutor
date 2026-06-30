package main

import (
	"path/filepath"
	"testing"
)

func TestDefaultDataDir_UsesXDGDataHome(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/tmp/ztutor-xdg")
	got := defaultDataDir()
	want := filepath.Join("/tmp/ztutor-xdg", "ztutor")
	if got != want {
		t.Fatalf("defaultDataDir = %q, want %q", got, want)
	}
}

func TestEnvOrDefault(t *testing.T) {
	t.Setenv("ZTUTOR_TEST_ENV", "from-env")
	if got := envOrDefault("ZTUTOR_TEST_ENV", "fallback"); got != "from-env" {
		t.Fatalf("envOrDefault with env = %q, want from-env", got)
	}
	if got := envOrDefault("ZTUTOR_MISSING_ENV", "fallback"); got != "fallback" {
		t.Fatalf("envOrDefault fallback = %q, want fallback", got)
	}
}
