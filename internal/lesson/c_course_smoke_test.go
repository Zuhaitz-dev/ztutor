package lesson

import (
	"path/filepath"
	"testing"
)

// TestCProgrammingCourse_Loads verifies that all lessons in the C course
// module 1 parse without error and satisfy basic structural invariants.
func TestCProgrammingCourse_Loads(t *testing.T) {
	root := filepath.Join("..", "..", "courses")
	courses, err := LoadCoursesLang(root, "en")
	if err != nil {
		t.Fatalf("LoadCoursesLang: %v", err)
	}

	var cCourse *Course
	for i := range courses {
		if courses[i].ID == "c-programming" {
			cCourse = &courses[i]
			break
		}
	}
	if cCourse == nil {
		t.Fatal("c-programming course not found")
	}

	if len(cCourse.Sections) == 0 {
		t.Fatal("c-programming course has no sections")
	}

	var lessons []Lesson
	for _, s := range cCourse.Sections {
		lessons = append(lessons, s.Lessons...)
	}

	// Module 1 has 16 entries: 1 intro + 15 exercise lessons.
	if len(lessons) < 16 {
		t.Fatalf("expected at least 16 lessons, got %d", len(lessons))
	}

	// Find the module intro lesson (00-module-intro).
	var intro *Lesson
	for i := range lessons {
		if lessons[i].ID == "00-module-intro" {
			intro = &lessons[i]
			break
		}
	}
	if intro == nil {
		t.Fatal("00-module-intro lesson not found")
	}
	if !intro.IsReadOnly() {
		t.Error("00-module-intro should be read-only (no exercise file)")
	}
	if intro.Content == "" {
		t.Error("00-module-intro has no content")
	}

	// Every exercise lesson (01-15) must have an exercise, content, and difficulty.
	for _, l := range lessons {
		if l.ID == "00-module-intro" {
			continue
		}
		if l.Content == "" {
			t.Errorf("lesson %s has no content", l.ID)
		}
		if l.Title == "" {
			t.Errorf("lesson %s has no title", l.ID)
		}
		if l.Difficulty == "" {
			t.Errorf("lesson %s has no difficulty set", l.ID)
		}
		if len(l.References) == 0 {
			t.Errorf("lesson %s has no references", l.ID)
		}
		if l.IsReadOnly() {
			t.Errorf("lesson %s should have an exercise but IsReadOnly() is true", l.ID)
		}
	}
}

// TestCProgrammingCourse_LoadsInAllLocales ensures the course loads cleanly
// for every supported UI language without frontmatter or file errors.
func TestCProgrammingCourse_LoadsInAllLocales(t *testing.T) {
	root := filepath.Join("..", "..", "courses")
	for _, lang := range []string{"en", "es", "ar", "zh"} {
		t.Run(lang, func(t *testing.T) {
			courses, err := LoadCoursesLang(root, lang)
			if err != nil {
				t.Fatalf("LoadCoursesLang(%s): %v", lang, err)
			}
			var found bool
			for _, c := range courses {
				if c.ID == "c-programming" {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("c-programming course not found for locale %s", lang)
			}
		})
	}
}
