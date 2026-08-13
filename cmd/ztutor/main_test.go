package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestCurrentUser_PriorityOrder(t *testing.T) {
	t.Setenv("ZTUTOR_USER", "bob")
	t.Setenv("USER", "alice")
	t.Setenv("LOGNAME", "carol")
	if got := currentUser(); got != "bob" {
		t.Errorf("currentUser = %q, want bob (ZTUTOR_USER wins)", got)
	}

	t.Setenv("ZTUTOR_USER", "")
	if got := currentUser(); got != "alice" {
		t.Errorf("currentUser = %q, want alice (USER)", got)
	}

	t.Setenv("USER", "")
	if got := currentUser(); got != "carol" {
		t.Errorf("currentUser = %q, want carol (LOGNAME)", got)
	}

	t.Setenv("LOGNAME", "")
	if got := currentUser(); got != "user" {
		t.Errorf("currentUser = %q, want fallback 'user'", got)
	}
}

func TestDefaultCoursesDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "courses"), 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if got := defaultCoursesDir(dir); got != filepath.Join(dir, "courses") {
		t.Errorf("defaultCoursesDir = %q, want installed dir", got)
	}
	if got := defaultCoursesDir(filepath.Join(dir, "missing")); got != "./courses" {
		t.Errorf("defaultCoursesDir = %q, want ./courses fallback", got)
	}
}
