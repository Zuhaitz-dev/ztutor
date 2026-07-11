package lesson

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func hasGCC() bool {
	_, err := exec.LookPath("gcc")
	return err == nil
}

func findCourseDirs(t *testing.T) []string {
	t.Helper()
	roots := []string{
		filepath.Join("..", "..", "courses"),
		filepath.Join("..", "courses"),
		"courses",
	}
	for _, root := range roots {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatalf("read courses dir %s: %v", root, err)
			}
			var dirs []string
			for _, e := range entries {
				if e.IsDir() {
					dirs = append(dirs, filepath.Join(root, e.Name()))
				}
			}
			return dirs
		}
	}
	t.Skip("courses directory not found")
	return nil
}

func findLessonDirs(t *testing.T) []string {
	t.Helper()
	courseDirs := findCourseDirs(t)
	var lessons []string
	for _, cd := range courseDirs {
		for _, sectionDir := range []string{"lessons", "interviews", "quizzes", "challenges"} {
			sec := filepath.Join(cd, sectionDir)
			entries, err := os.ReadDir(sec)
			if err != nil {
				continue
			}
			for _, e := range entries {
				if e.IsDir() {
					dir := filepath.Join(sec, e.Name())
					if _, err := os.Stat(filepath.Join(dir, "lesson.md")); err == nil {
						lessons = append(lessons, dir)
					}
				}
			}
		}
	}
	if len(lessons) == 0 {
		t.Skip("no lesson directories found")
	}
	return lessons
}

func TestContent_HeadersCompile(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	for _, dir := range findLessonDirs(t) {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Errorf("%s: read dir: %v", dir, err)
			continue
		}
		for _, e := range entries {
			if !strings.HasSuffix(e.Name(), ".h") {
				continue
			}
			header := filepath.Join(dir, e.Name())
			cmd := exec.Command("gcc", "-fsyntax-only", "-x", "c", "-include", "stdint.h", header)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Errorf("%s: does not compile:\n%s", header, strings.TrimSpace(string(out)))
			}
		}
	}
}

func TestContent_ExpectedOutputExists(t *testing.T) {
	for _, dir := range findLessonDirs(t) {
		path := filepath.Join(dir, "expected.txt")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Errorf("%s: read: %v", path, err)
			continue
		}
		if strings.TrimSpace(string(data)) == "" {
			t.Errorf("%s: expected.txt is empty", path)
		}
	}
}

func TestContent_IncludesExist(t *testing.T) {
	includeRE := regexp.MustCompile(`#include\s+"([^"]+)"`)
	for _, dir := range findLessonDirs(t) {
		exercisePath := filepath.Join(dir, "exercise.c")
		data, err := os.ReadFile(exercisePath)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Errorf("%s: read: %v", exercisePath, err)
			continue
		}
		for _, match := range includeRE.FindAllStringSubmatch(string(data), -1) {
			includeFile := match[1]
			target := filepath.Join(dir, includeFile)
			if _, err := os.Stat(target); os.IsNotExist(err) {
				t.Errorf("%s: #include %q: file not found in lesson directory", exercisePath, includeFile)
			}
		}
	}
}

func TestContent_FrontmatterValid(t *testing.T) {
	type fm struct {
		Difficulty string   `yaml:"difficulty"`
		Tags       []string `yaml:"tags"`
		Premium    bool     `yaml:"premium"`
	}

	for _, dir := range findLessonDirs(t) {
		path := filepath.Join(dir, "lesson.md")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Errorf("%s: read: %v", path, err)
			continue
		}
		front, _, err := extractFrontMatter(string(data))
		if err != nil {
			t.Errorf("%s: frontmatter parse: %v", path, err)
			continue
		}
		if front == "" {
			t.Errorf("%s: missing YAML frontmatter", path)
			continue
		}
		var f fm
		if err := yaml.Unmarshal([]byte(front), &f); err != nil {
			t.Errorf("%s: frontmatter YAML: %v", path, err)
			continue
		}
		if f.Difficulty == "" {
			t.Errorf("%s: missing difficulty field", path)
		}
		if len(f.Tags) == 0 {
			t.Errorf("%s: missing tags field", path)
		}
	}
}

func extractFrontMatter(content string) (front, body string, err error) {
	content = strings.TrimSpace(content)
	if !strings.HasPrefix(content, "---") {
		return "", content, nil
	}
	parts := strings.SplitN(content[3:], "---", 2)
	if len(parts) != 2 {
		return "", content, nil
	}
	return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1]), nil
}
