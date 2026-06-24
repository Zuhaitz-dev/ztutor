package lesson

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAll_MissingDir(t *testing.T) {
	_, err := LoadAll("/nonexistent/path/that/does/not/exist")
	if err == nil {
		t.Error("expected error for missing directory, got nil")
	}
}

func TestLoadAll_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	lessons, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll empty dir: %v", err)
	}
	if len(lessons) != 0 {
		t.Errorf("lessons = %d, want 0", len(lessons))
	}
}

func TestLoadAll_SkipsBrokenLesson(t *testing.T) {
	dir := t.TempDir()

	// Good lesson.
	good := filepath.Join(dir, "01-good")
	os.MkdirAll(good, 0755)
	os.WriteFile(filepath.Join(good, "lesson.md"), []byte("# Good Lesson\n\nContent."), 0644)

	// Broken: dir exists but lesson.md is unreadable (simulate by making it a directory).
	bad := filepath.Join(dir, "02-bad")
	os.MkdirAll(bad, 0755)
	os.MkdirAll(filepath.Join(bad, "lesson.md"), 0755) // dir instead of file → ReadFile fails

	lessons, err := LoadAll(dir)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}
	if len(lessons) != 1 {
		t.Errorf("lessons = %d, want 1 (broken lesson skipped)", len(lessons))
	}
	if lessons[0].ID != "01-good" {
		t.Errorf("lesson ID = %q, want 01-good", lessons[0].ID)
	}
}

func TestLoadCourse_MissingSectionDir(t *testing.T) {
	dir := t.TempDir()

	manifest := `id: test-course
title: Test Course
language: c
order: 1
toolchain:
  source_extension: .c
  syntax_highlighting: c
sections:
  - id: missing
    title: Missing Section
    type: exercises
    dir: lessons/does-not-exist
`
	os.WriteFile(filepath.Join(dir, "course.yaml"), []byte(manifest), 0644)

	c, err := LoadCourseLang(dir, "en")
	if err != nil {
		t.Fatalf("LoadCourse: %v", err)
	}
	// Section is created but has no lessons because dir was missing.
	if len(c.Sections) == 0 {
		t.Fatal("expected section to be created even with missing dir")
	}
	if len(c.Sections[0].Lessons) != 0 {
		t.Errorf("missing section dir should produce 0 lessons, got %d", len(c.Sections[0].Lessons))
	}
}

func TestLoadCourse_MultiSection(t *testing.T) {
	dir := t.TempDir()

	sec1 := filepath.Join(dir, "lessons", "basics")
	sec2 := filepath.Join(dir, "lessons", "advanced")
	os.MkdirAll(sec1, 0755)
	os.MkdirAll(sec2, 0755)

	l1 := filepath.Join(sec1, "01-hello")
	os.MkdirAll(l1, 0755)
	os.WriteFile(filepath.Join(l1, "lesson.md"), []byte("# Hello\n\nBasic."), 0644)

	l2 := filepath.Join(sec2, "01-pointers")
	os.MkdirAll(l2, 0755)
	os.WriteFile(filepath.Join(l2, "lesson.md"), []byte("# Pointers\n\nAdvanced."), 0644)

	manifest := `id: c-programming
title: C Programming
language: c
order: 1
toolchain:
  source_extension: .c
  syntax_highlighting: c
sections:
  - id: basics
    title: Basics
    type: exercises
    dir: lessons/basics
  - id: advanced
    title: Advanced
    type: exercises
    dir: lessons/advanced
`
	os.WriteFile(filepath.Join(dir, "course.yaml"), []byte(manifest), 0644)

	c, err := LoadCourseLang(dir, "en")
	if err != nil {
		t.Fatalf("LoadCourse: %v", err)
	}
	if len(c.Sections) != 2 {
		t.Fatalf("sections = %d, want 2", len(c.Sections))
	}
	if len(c.Sections[0].Lessons) != 1 {
		t.Errorf("basics lessons = %d, want 1", len(c.Sections[0].Lessons))
	}
	if len(c.Sections[1].Lessons) != 1 {
		t.Errorf("advanced lessons = %d, want 1", len(c.Sections[1].Lessons))
	}
	if c.TotalLessons != 2 {
		t.Errorf("TotalLessons = %d, want 2", c.TotalLessons)
	}
	// Language propagated to all lessons.
	for _, s := range c.Sections {
		for _, l := range s.Lessons {
			if l.Language != "c" {
				t.Errorf("lesson %s language = %q, want c", l.ID, l.Language)
			}
		}
	}
}

func TestLoadCourse_Defaults(t *testing.T) {
	dir := t.TempDir()
	// No manifest — legacy mode.
	l1 := filepath.Join(dir, "01-hello")
	os.MkdirAll(l1, 0755)
	os.WriteFile(filepath.Join(l1, "lesson.md"), []byte("# Hello\n\nContent."), 0644)

	c, err := LoadCourseLang(dir, "en")
	if err != nil {
		t.Fatalf("LoadCourse: %v", err)
	}
	if c.Language != "c" {
		t.Errorf("default language = %q, want c", c.Language)
	}
	if c.SourceExtension != ".c" {
		t.Errorf("default source_extension = %q, want .c", c.SourceExtension)
	}
	if len(c.Sections) != 1 {
		t.Fatalf("sections = %d, want 1 (legacy single section)", len(c.Sections))
	}
}

func TestLoadCourse_PremiumFlagSetsFreemium(t *testing.T) {
	dir := t.TempDir()
	sec := filepath.Join(dir, "lessons")
	os.MkdirAll(sec, 0755)

	free := filepath.Join(sec, "01-free")
	os.MkdirAll(free, 0755)
	os.WriteFile(filepath.Join(free, "lesson.md"), []byte("# Free\n\nContent."), 0644)

	premium := filepath.Join(sec, "02-premium")
	os.MkdirAll(premium, 0755)
	os.WriteFile(filepath.Join(premium, "lesson.md"), []byte("---\npremium: true\n---\n# Premium\n\nContent."), 0644)

	manifest := `id: test
title: Test
language: c
order: 1
toolchain:
  source_extension: .c
sections:
  - id: main
    title: Main
    type: exercises
    dir: lessons
`
	os.WriteFile(filepath.Join(dir, "course.yaml"), []byte(manifest), 0644)

	c, err := LoadCourseLang(dir, "en")
	if err != nil {
		t.Fatalf("LoadCourse: %v", err)
	}
	if !c.HasFreemium {
		t.Error("HasFreemium = false, want true (course has a premium lesson)")
	}
}
