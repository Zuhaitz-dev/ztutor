package main

import (
	"os/exec"
	"path/filepath"
	"strings"
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

func TestVersionFlag(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "--version").CombinedOutput()
	if err != nil {
		t.Fatalf("--version failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "ztutor") {
		t.Errorf("--version output = %q, want to contain 'ztutor'", string(out))
	}
}

func TestCheckUpdateFlag(t *testing.T) {
	out, err := exec.Command("go", "run", ".", "--check-update").CombinedOutput()
	if err != nil {
		t.Fatalf("--check-update failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "up to date") {
		t.Errorf("--check-update output = %q, want 'up to date'", string(out))
	}
}
