package lesson

import (
	"path/filepath"
	"reflect"
	"strings"
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
				if courses[i].ID == "ztutor-starter" {
					starter = &courses[i]
					break
				}
			}
			if starter == nil {
				t.Fatal("starter course not loaded")
			}
			if len(starter.CourseIntro) == 0 {
				t.Fatal("starter course has no localized intro")
			}
			if len(starter.Sections) != 2 || len(starter.Sections[0].Lessons) != 4 || len(starter.Sections[1].Quizzes) != 1 {
				t.Fatalf("starter structure = %+v, want lessons section with four lessons and quiz section with one quiz", starter.Sections)
			}
			if starter.TotalQuizzes != 1 {
				t.Fatalf("starter total quizzes = %d, want 1", starter.TotalQuizzes)
			}
			wantProgLangs := []string{"c", "python", "go", "rust"}
			if !reflect.DeepEqual(starter.ProgrammingLanguages, wantProgLangs) {
				t.Fatalf("starter programming languages = %v, want %v", starter.ProgrammingLanguages, wantProgLangs)
			}
			wantOrder := []string{
				"01-toolchain-check",
				"02-input-greeting",
				"03-go-hello",
				"04-rust-hello",
			}
			wantLang := map[string]string{
				"01-toolchain-check": "c",
				"02-input-greeting":  "python",
				"03-go-hello":        "go",
				"04-rust-hello":      "rust",
			}
			gotOrder := make([]string, 0, len(starter.Sections[0].Lessons))
			for _, lesson := range starter.Sections[0].Lessons {
				gotOrder = append(gotOrder, lesson.ID)
				if lesson.Exercise == "" {
					t.Fatalf("%s has no exercise", lesson.ID)
				}
				if len(lesson.Tests) != 1 || strings.TrimSpace(lesson.Tests[0].Expected) != "hello world" {
					t.Fatalf("%s expected output = %+v", lesson.ID, lesson.Tests)
				}
				if len(lesson.Hints) == 0 {
					t.Fatalf("%s has no localized hints", lesson.ID)
				}
				if wantLang[lesson.ID] != lesson.Language {
					t.Fatalf("%s language = %q, want %q", lesson.ID, lesson.Language, wantLang[lesson.ID])
				}
			}
			if !reflect.DeepEqual(gotOrder, wantOrder) {
				t.Fatalf("lesson order = %v, want %v", gotOrder, wantOrder)
			}
			quiz := starter.Sections[1].Quizzes[0]
			if quiz.ID != "05-hello-world-review" {
				t.Fatalf("quiz id = %q", quiz.ID)
			}
			if len(quiz.Questions) != 3 {
				t.Fatalf("quiz question count = %d, want 3", len(quiz.Questions))
			}
			for _, q := range quiz.Questions {
				if q.Prompt == "" || q.Explanation == "" {
					t.Fatalf("quiz question %s missing localized prompt or explanation", q.ID)
				}
				correct := 0
				for _, opt := range q.Options {
					if opt.Text == "" {
						t.Fatalf("quiz question %s has empty option text", q.ID)
					}
					if opt.Correct {
						correct++
					}
				}
				if correct != 1 {
					t.Fatalf("quiz question %s correct option count = %d, want 1", q.ID, correct)
				}
			}
		})
	}
}
