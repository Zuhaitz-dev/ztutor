package sandbox

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteFiles_RejectsDoubleDot(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"../../etc/passwd": "hacked",
	}
	err := writeFiles(dir, files)
	if err == nil {
		t.Error("writeFiles should reject '..' in filenames")
	}
}

func TestWriteFiles_RejectsAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"/etc/passwd": "hacked",
	}
	err := writeFiles(dir, files)
	if err == nil {
		t.Error("writeFiles should reject absolute paths")
	}
}

func TestWriteFiles_RejectsEmptyName(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"": "content",
	}
	err := writeFiles(dir, files)
	if err == nil {
		t.Error("writeFiles should reject empty filenames")
	}
}

func TestWriteFiles_RejectsTraversalViaSubdir(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"subdir/../../../etc/passwd": "hacked",
	}
	err := writeFiles(dir, files)
	if err == nil {
		t.Error("writeFiles should reject traversal via subdirectory")
	}
}

func TestWriteFiles_AcceptsValidNames(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"main.c":    "int main() { return 0; }",
		"utils.h":   "void helper(void);",
		"sub/dir.c": "// nested",
	}
	err := writeFiles(dir, files)
	if err != nil {
		t.Fatalf("writeFiles valid files: %v", err)
	}
	for name, content := range files {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err != nil {
			t.Errorf("ReadFile %s: %v", p, err)
			continue
		}
		if string(data) != content {
			t.Errorf("file %s content mismatch: got %q, want %q", name, string(data), content)
		}
	}
}

func TestSafeWritePath_RejectsDoubleDot(t *testing.T) {
	_, err := safeWritePath("/tmp/sandbox", "../../etc/passwd")
	if err == nil {
		t.Error("safeWritePath should reject '..'")
	}
}

func TestSafeWritePath_RejectsAbsolute(t *testing.T) {
	_, err := safeWritePath("/tmp/sandbox", "/etc/passwd")
	if err == nil {
		t.Error("safeWritePath should reject absolute paths")
	}
}

func TestSafeWritePath_RejectsEmpty(t *testing.T) {
	_, err := safeWritePath("/tmp/sandbox", "")
	if err == nil {
		t.Error("safeWritePath should reject empty name")
	}
}

func TestSafeWritePath_AcceptsValid(t *testing.T) {
	path, err := safeWritePath("/tmp/sandbox", "main.c")
	if err != nil {
		t.Fatalf("safeWritePath valid: %v", err)
	}
	if path != "/tmp/sandbox/main.c" {
		t.Errorf("safeWritePath returned %q, want /tmp/sandbox/main.c", path)
	}
}

func TestSandboxEnv_DoesNotLeakParentEnv(t *testing.T) {
	os.Setenv("ZTUTOR_TEST_LEAK", "secret")
	defer os.Unsetenv("ZTUTOR_TEST_LEAK")

	env := sandboxEnv("/tmp/sandbox", nil)
	for _, e := range env {
		if len(e) > 6 && e[:4] == "AWS_" {
			t.Errorf("sandboxEnv should not leak AWS_* variables, got %q", e)
		}
		if len(e) > 5 && e[:4] == "XDG_" {
			t.Errorf("sandboxEnv should not leak XDG_* variables, got %q", e)
		}
		// The parent test variable should not be present.
		if len(e) > 17 && e[:17] == "ZTUTOR_TEST_LEAK" {
			t.Errorf("sandboxEnv should not leak custom test variables, got %q", e)
		}
	}
}

func TestSandboxEnv_ContainsRequiredVars(t *testing.T) {
	env := sandboxEnv("/tmp/sandbox", nil)
	envMap := make(map[string]string, len(env))
	for _, e := range env {
		for i := 0; i < len(e); i++ {
			if e[i] == '=' {
				envMap[e[:i]] = e[i+1:]
				break
			}
		}
	}
	required := []string{"PATH", "HOME", "TMPDIR", "LANG"}
	for _, key := range required {
		if _, ok := envMap[key]; !ok {
			t.Errorf("sandboxEnv missing required variable %s", key)
		}
	}
	if envMap["HOME"] != "/tmp/sandbox" {
		t.Errorf("HOME = %q, want /tmp/sandbox", envMap["HOME"])
	}
}

func TestSandboxEnv_WithExtraVars(t *testing.T) {
	env := sandboxEnv("/tmp/sandbox", []string{"ASAN_OPTIONS=detect_leaks=0", "CUSTOM=value"})
	hasAsan := false
	hasCustom := false
	for _, e := range env {
		if e == "ASAN_OPTIONS=detect_leaks=0" {
			hasAsan = true
		}
		if e == "CUSTOM=value" {
			hasCustom = true
		}
	}
	if !hasAsan {
		t.Error("sandboxEnv should include extra env vars (ASAN_OPTIONS)")
	}
	if !hasCustom {
		t.Error("sandboxEnv should include extra env vars (CUSTOM)")
	}
}
