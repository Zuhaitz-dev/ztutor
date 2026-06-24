package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Keymap != "default" {
		t.Errorf("Keymap = %q, want default", cfg.Keymap)
	}
	if cfg.SSH.Addr != ":2222" {
		t.Errorf("SSH.Addr = %q, want :2222", cfg.SSH.Addr)
	}
	if cfg.SSH.HostKey != "ztutor_host_key" {
		t.Errorf("SSH.HostKey = %q, want ztutor_host_key", cfg.SSH.HostKey)
	}
	if cfg.DB.Path != "ztutor.db" {
		t.Errorf("DB.Path = %q, want ztutor.db", cfg.DB.Path)
	}
	if cfg.CoursesDir != "./courses" {
		t.Errorf("CoursesDir = %q, want ./courses", cfg.CoursesDir)
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load("/nonexistent/ztutor.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Keymap != "default" {
		t.Errorf("missing file should return defaults, Keymap = %q", cfg.Keymap)
	}
}

func TestLoad_ValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ztutor.json")
	content := `{
		"keymap": "vim",
		"ssh": {"addr": ":3333", "host_key": "/tmp/key"},
		"db": {"path": "/tmp/db.sqlite"},
		"courses_dir": "/tmp/courses"
	}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Keymap != "vim" {
		t.Errorf("Keymap = %q, want vim", cfg.Keymap)
	}
	if cfg.SSH.Addr != ":3333" {
		t.Errorf("SSH.Addr = %q, want :3333", cfg.SSH.Addr)
	}
	if cfg.DB.Path != "/tmp/db.sqlite" {
		t.Errorf("DB.Path = %q, want /tmp/db.sqlite", cfg.DB.Path)
	}
	if cfg.CoursesDir != "/tmp/courses" {
		t.Errorf("CoursesDir = %q, want /tmp/courses", cfg.CoursesDir)
	}
}

func TestLoad_MalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ztutor.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Error("malformed JSON should return an error")
	}
}

func TestLoad_PartialJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ztutor.json")
	content := `{"keymap": "vim"}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Keymap != "vim" {
		t.Errorf("Keymap = %q, want vim", cfg.Keymap)
	}
	// Unset fields should retain defaults
	if cfg.SSH.Addr != ":2222" {
		t.Errorf("SSH.Addr = %q, want :2222 (default)", cfg.SSH.Addr)
	}
}

func TestLoad_ExecSection(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ztutor.json")
	content := `{"exec": {"addr": ":9999"}}`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Exec.Addr != ":9999" {
		t.Errorf("Exec.Addr = %q, want :9999", cfg.Exec.Addr)
	}
}
