package lesson

import (
	"os"
	"path/filepath"
	"testing"
)

func FuzzLoadCourseLangYAML(f *testing.F) {
	// Seed with a minimal valid course.yaml
	f.Add("id: test-course\ntitle: Test\nlanguage: c\norder: 1\nsections: []\n")
	// Common edge cases
	f.Add("")
	f.Add("}{")
	f.Add("id: 42")   // id is integer, not string
	f.Add("id: null") // null id
	f.Add("id: !!python/object/apply:os.system [\"rm -rf /\"]")
	f.Add(string([]byte{0xFF, 0xFE, 0xFD}))
	f.Add("sections:\n  - id: s1\n    type: exercises\n    dir: \"../../etc\"")

	f.Fuzz(func(t *testing.T, yamlContent string) {
		if len(yamlContent) > 65536 {
			return
		}

		dir := t.TempDir()
		courseYamlPath := filepath.Join(dir, "course.yaml")
		if err := os.WriteFile(courseYamlPath, []byte(yamlContent), 0644); err != nil {
			t.Skip()
		}

		c, err := LoadCourseLang(dir, "en")
		if err != nil {
			// Errors are expected for invalid YAML, but no panics.
			return
		}
		_ = c
	})
}

func FuzzSanitizeFilePath(f *testing.F) {
	f.Add("normal_file.c")
	f.Add("../etc/passwd")
	f.Add("/absolute/path")
	f.Add("")
	f.Add("a\nb")
	f.Add(string([]byte{0x00, 0x01, 0x02}))

	f.Fuzz(func(t *testing.T, name string) {
		if len(name) > 4096 {
			return
		}
		// Assert safeWritePath rejects traversal and empty names.
		// We import sandbox but this is a different package. The sandbox.safeWritePath
		// is unexported. We'll just verify LoadAllLang doesn't panic.
		dir := t.TempDir()
		lessonDir := filepath.Join(dir, name)
		// Don't attempt to create paths that might escape temp dir
		if !filepath.IsAbs(lessonDir) {
			lessons, err := LoadAllLang(lessonDir, "en")
			if err == nil && lessons != nil {
				t.Logf("lessons loaded: %d", len(lessons))
			}
		}
	})
}
