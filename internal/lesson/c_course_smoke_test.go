package lesson

import (
	"path/filepath"
	"testing"
)

func loadCCourse(t *testing.T) *Course {
	t.Helper()
	root := filepath.Join("..", "..", "courses")
	courses, err := LoadCoursesLang(root, "en")
	if err != nil {
		t.Fatalf("LoadCoursesLang: %v", err)
	}
	for i := range courses {
		if courses[i].ID == "c-programming" {
			return &courses[i]
		}
	}
	t.Fatal("c-programming course not found")
	return nil
}

func lessonsOf(c *Course) []Lesson {
	var lessons []Lesson
	for _, s := range c.Sections {
		lessons = append(lessons, s.Lessons...)
	}
	return lessons
}

func lessonByID(t *testing.T, lessons []Lesson, id string) *Lesson {
	t.Helper()
	for i := range lessons {
		if lessons[i].ID == id {
			return &lessons[i]
		}
	}
	t.Fatalf("lesson %s not found", id)
	return nil
}

func TestCProgrammingCourse_Loads(t *testing.T) {
	cCourse := loadCCourse(t)

	if len(cCourse.Sections) == 0 {
		t.Fatal("c-programming course has no sections")
	}

	lessons := lessonsOf(cCourse)

	if len(lessons) < 16 {
		t.Fatalf("expected at least 16 lessons, got %d", len(lessons))
	}

	intro := lessonByID(t, lessons, "00-module-intro")
	if !intro.IsReadOnly() {
		t.Error("00-module-intro should be read-only (no exercise file)")
	}
	if intro.Content == "" {
		t.Error("00-module-intro has no content")
	}

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

func TestCProgrammingCourse_LessonMetadata(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))

	tests := []struct {
		id          string
		wantDiff    string
		hasExercise bool
		wantLang    string
	}{
		{id: "00-module-intro", wantDiff: "beginner", hasExercise: false, wantLang: "c"},
		{id: "01-hello-stderr", wantDiff: "beginner", hasExercise: true, wantLang: "c"},
		{id: "02-modern-integers", wantDiff: "beginner", hasExercise: true, wantLang: "c"},
		{id: "03-scope-shadowing", wantDiff: "beginner", hasExercise: true, wantLang: "c"},
		{id: "04-standard-io-traps", wantDiff: "beginner", hasExercise: true, wantLang: "c"},
		{id: "05-booleans", wantDiff: "beginner", hasExercise: true, wantLang: "c"},
		{id: "06-duffs-device", wantDiff: "intermediate", hasExercise: true, wantLang: "c"},
		{id: "07-for-loops-cache", wantDiff: "intermediate", hasExercise: true, wantLang: "c"},
		{id: "08-while-eof", wantDiff: "beginner", hasExercise: true, wantLang: "c"},
		{id: "09-call-stack", wantDiff: "intermediate", hasExercise: true, wantLang: "c"},
		{id: "10-pass-by-value", wantDiff: "intermediate", hasExercise: true, wantLang: "c"},
		{id: "11-preprocessor-macros", wantDiff: "intermediate", hasExercise: true, wantLang: "c"},
		{id: "12-conditional-compilation", wantDiff: "advanced", hasExercise: true, wantLang: "c"},
		{id: "13-macro-gotchas", wantDiff: "advanced", hasExercise: true, wantLang: "c"},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			l := lessonByID(t, lessons, tt.id)
			if l.Difficulty != tt.wantDiff {
				t.Errorf("Difficulty = %q, want %q", l.Difficulty, tt.wantDiff)
			}
			if l.Language != tt.wantLang {
				t.Errorf("Language = %q, want %q", l.Language, tt.wantLang)
			}
			if tt.hasExercise && l.Exercise == "" {
				t.Error("Exercise is empty, expected exercise.c")
			}
			if !tt.hasExercise && l.ID != "00-module-intro" {
				if !l.IsReadOnly() {
					t.Error("expected read-only lesson")
				}
			}
		})
	}
}

func TestCProgrammingCourse_TestCases(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))

	for _, l := range lessons {
		if l.ID == "00-module-intro" {
			continue
		}
		t.Run(l.ID, func(t *testing.T) {
			if len(l.Tests) == 0 {
				t.Error("lesson has no test cases")
			}
			for i, tc := range l.Tests {
				if tc.HasExpectedStdout && tc.ExpectedStdout == "" {
					t.Errorf("test %d: HasExpectedStdout=true but ExpectedStdout is empty", i+1)
				}
				if tc.HasExpectedStderr && tc.ExpectedStderr == "" {
					t.Errorf("test %d: HasExpectedStderr=true but ExpectedStderr is empty", i+1)
				}
				if !tc.HasExpectedStdout && !tc.HasExpectedStderr && tc.Expected == "" {
					t.Errorf("test %d: no expected output at all", i+1)
				}
			}
		})
	}
}

func TestCProgrammingCourse_HintsAndTrivia(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))

	for _, l := range lessons {
		if l.ID == "00-module-intro" {
			continue
		}
		t.Run(l.ID, func(t *testing.T) {
			if len(l.Hints) == 0 {
				t.Error("lesson has no hints")
			}
			if len(l.Trivia) == 0 {
				t.Error("lesson has no trivia")
			}
			for i, h := range l.Hints {
				if h == "" {
					t.Errorf("hint %d is empty", i+1)
				}
			}
			for i, tr := range l.Trivia {
				if tr == "" {
					t.Errorf("trivia %d is empty", i+1)
				}
			}
		})
	}
}

func TestMultiFileLesson_Metadata(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))
	l := lessonByID(t, lessons, "14-makefiles-101")

	if l.BuildCmd != "make" {
		t.Errorf("BuildCmd = %q, want %q", l.BuildCmd, "make")
	}
	if l.BuildOutput != "redis-server" {
		t.Errorf("BuildOutput = %q, want %q", l.BuildOutput, "redis-server")
	}
	if l.Language != "c" {
		t.Errorf("Language = %q, want %q (Bug #1: should propagate from course)", l.Language, "c")
	}
	if len(l.Files) != 3 {
		t.Fatalf("Files = %d, want 3", len(l.Files))
	}

	if l.Files[0].Name != "Makefile" {
		t.Errorf("Files[0].Name = %q, want Makefile", l.Files[0].Name)
	}
	if !l.Files[0].Editable {
		t.Error("Files[0] (Makefile) should be editable")
	}

	var serverFile *ExerciseFile
	var zmallocFile *ExerciseFile
	for i := range l.Files {
		switch l.Files[i].Name {
		case "server.c":
			serverFile = &l.Files[i]
		case "zmalloc.c":
			zmallocFile = &l.Files[i]
		}
	}

	if serverFile == nil {
		t.Fatal("server.c not found in multi-file lesson")
	}
	if serverFile.Editable {
		t.Error("server.c Editable = true, want false (frontmatter sets editable: false)")
	}
	if serverFile.Language != "c" {
		t.Errorf("server.c Language = %q, want c", serverFile.Language)
	}
	if serverFile.Content == "" {
		t.Error("server.c has no content")
	}

	if zmallocFile == nil {
		t.Fatal("zmalloc.c not found in multi-file lesson")
	}
	if zmallocFile.Editable {
		t.Error("zmalloc.c Editable = true, want false (frontmatter sets editable: false)")
	}

	if len(l.Tests) == 0 {
		t.Error("multi-file lesson has no test cases")
	}
}

func TestStderrLesson_StreamAware(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))
	l := lessonByID(t, lessons, "01-hello-stderr")

	if len(l.Tests) == 0 {
		t.Fatal("no test cases")
	}
	tc := l.Tests[0]
	if !tc.HasExpectedStdout {
		t.Error("HasExpectedStdout is false, lesson 01 should have expected.stdout.txt")
	}
	if !tc.HasExpectedStderr {
		t.Error("HasExpectedStderr is false, lesson 01 should have expected.stderr.txt")
	}
	if tc.ExpectedStdout != "SHA1_STREAM_BLOB: 0x7a2f5b9c\n" {
		t.Errorf("ExpectedStdout = %q, want %q", tc.ExpectedStdout, "SHA1_STREAM_BLOB: 0x7a2f5b9c\n")
	}
	if tc.ExpectedStderr == "" {
		t.Error("ExpectedStderr is empty")
	}
}

func TestCapstoneLesson_Metadata(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))
	l := lessonByID(t, lessons, "15-capstone-redis")

	if len(l.Tests) == 0 {
		t.Fatal("no test cases")
	}
	tc := l.Tests[0]
	if tc.Stdin == "" {
		t.Error("expected stdin.txt for capstone lesson")
	}
	if tc.Expected == "" {
		t.Error("expected expected.txt for capstone lesson")
	}
	if l.Language != "c" {
		t.Errorf("Language = %q, want c", l.Language)
	}
}

func TestLessonLanguage_Propagation(t *testing.T) {
	lessons := lessonsOf(loadCCourse(t))

	for _, l := range lessons {
		if l.ID == "00-module-intro" {
			continue
		}
		if l.Language == "" {
			t.Errorf("lesson %s: Language is empty, should be 'c' (propagated from course)", l.ID)
		}
	}
}

func TestCourseMetadata(t *testing.T) {
	c := loadCCourse(t)

	if c.ID != "c-programming" {
		t.Errorf("Course ID = %q, want c-programming", c.ID)
	}
	if c.Language != "c" {
		t.Errorf("Course Language = %q, want c", c.Language)
	}
	if c.Layout != CourseLayoutPath {
		t.Errorf("Course Layout = %q, want path", c.Layout)
	}
	if c.EnrollmentRequired {
		t.Error("c-programming should have enrollment required = false")
	}
	if c.TotalLessons != 16 {
		t.Errorf("TotalLessons = %d, want 16", c.TotalLessons)
	}
}
