package lesson

import (
	"path/filepath"
	"testing"
)

func TestStarterCourseLoadsInAllUILanguages(t *testing.T) {
	root := filepath.Join("..", "..", "courses")
	for _, lang := range []string{"en", "es", "zh", "ar"} {
		t.Run(lang, func(t *testing.T) {
			courses, err := LoadCoursesLang(root, lang)
			if err != nil {
				t.Fatalf("load courses: %v", err)
			}
			var starter *Course
			for i := range courses {
				if courses[i].ID == "c-programming" {
					starter = &courses[i]
					break
				}
			}
			if starter == nil {
				t.Fatal("c-programming course not loaded")
			}
			if starter.EnrollmentRequired {
				t.Fatal("c-programming should be open access (enrollment not required)")
			}
			if len(starter.Sections) != 1 {
				t.Fatalf("section count = %d, want 1", len(starter.Sections))
			}
			sec := starter.Sections[0]
			if len(sec.Lessons) != 16 {
				t.Fatalf("lesson count = %d, want 16", len(sec.Lessons))
			}
			for _, lesson := range sec.Lessons {
				if lesson.ID == "00-module-intro" {
					continue // intro page has no exercise
				}
				if lesson.Exercise == "" && lesson.ID != "14-makefiles-101" {
					t.Errorf("lesson %s has no exercise file", lesson.ID)
				}
				if len(lesson.Tests) == 0 {
					t.Errorf("lesson %s has no test cases", lesson.ID)
				}
			}
		})
	}
}
