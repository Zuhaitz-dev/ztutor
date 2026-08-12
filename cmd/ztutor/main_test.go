package main

import (
	"os/exec"
	"strings"
	"testing"
)

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
	// --check-update in dev mode should say "up to date".
	out, err := exec.Command("go", "run", ".", "--check-update").CombinedOutput()
	if err != nil {
		t.Fatalf("--check-update failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "up to date") {
		t.Errorf("--check-update output = %q, want 'up to date'", string(out))
	}
}
