package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
)

func TestCourseIntroBeats_CustomBeatsUsed(t *testing.T) {
	loc := i18n.New("en")
	custom := []string{"Hello.", "Let's go."}
	beats, metas := courseIntroBeats("go", "My Course", custom, loc)
	if len(beats) != 2 {
		t.Fatalf("expected 2 beats from custom, got %d", len(beats))
	}
	if len(metas) != 2 {
		t.Fatalf("expected 2 metas, got %d", len(metas))
	}
	if !strings.Contains(beats[0].Text, "Hello.") {
		t.Errorf("beat[0] = %q, want to contain custom text", beats[0].Text)
	}
	// Last beat must have AutoDelay 0 (holds until user advances).
	if beats[len(beats)-1].AutoDelay != 0 {
		t.Errorf("last beat AutoDelay = %v, want 0", beats[len(beats)-1].AutoDelay)
	}
}

func TestCourseIntroBeats_GoLang(t *testing.T) {
	loc := i18n.New("en")
	beats, _ := courseIntroBeats("go", "Go Course", nil, loc)
	if len(beats) == 0 {
		t.Fatal("expected locale-based beats for lang=go, got none")
	}
	if !strings.Contains(beats[0].Text, loc.T("intro.go.beat1")) {
		t.Errorf("beat[0] = %q, want intro.go.beat1 content", beats[0].Text)
	}
}

func TestCourseIntroBeats_RustLang(t *testing.T) {
	loc := i18n.New("en")
	beats, _ := courseIntroBeats("rust", "Rust Course", nil, loc)
	if len(beats) == 0 {
		t.Fatal("expected locale-based beats for lang=rust, got none")
	}
	if !strings.Contains(beats[0].Text, loc.T("intro.rust.beat1")) {
		t.Errorf("beat[0] = %q, want intro.rust.beat1 content", beats[0].Text)
	}
}

func TestCourseIntroBeats_PythonNormalized(t *testing.T) {
	loc := i18n.New("en")
	// "python" should normalise to "py" so intro.py.beat* keys are used.
	beats, _ := courseIntroBeats("python", "Python Course", nil, loc)
	if len(beats) == 0 {
		t.Fatal("expected locale-based beats for lang=python, got none")
	}
	if !strings.Contains(beats[0].Text, loc.T("intro.py.beat1")) {
		t.Errorf("beat[0] = %q, want intro.py.beat1 content", beats[0].Text)
	}
}

func TestCourseIntroBeats_UnknownLangFallsToDefault(t *testing.T) {
	loc := i18n.New("en")
	beats, _ := courseIntroBeats("cobol", "COBOL Course", nil, loc)
	if len(beats) == 0 {
		t.Fatal("expected default fallback beats for unknown lang, got none")
	}
	// Default beat1 uses the title as %s.
	if !strings.Contains(beats[0].Text, "COBOL Course") {
		t.Errorf("default fallback beat[0] = %q, want title %q in text", beats[0].Text, "COBOL Course")
	}
}

func TestCourseIntroBeats_LocalesAllHaveGoRust(t *testing.T) {
	for _, lang := range []string{"en", "es", "ar", "zh"} {
		loc := i18n.New(lang)
		for _, courseLang := range []string{"go", "rust"} {
			beats, _ := courseIntroBeats(courseLang, "Test", nil, loc)
			if len(beats) == 0 {
				t.Errorf("lang=%s courseLang=%s: expected beats, got none", lang, courseLang)
			}
		}
	}
}

func TestKeybindingsOverlay_NoGamepadShowsFKeys(t *testing.T) {
	loc := i18n.New("en")
	overlay := newKeybindingsOverlay(loc)
	overlay.visible = true
	lines := overlay.allLines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "F13") {
		t.Errorf("without gamepad, overlay should show F13; got:\n%s", joined)
	}
	if strings.Contains(joined, "A / Cross") {
		t.Errorf("without gamepad, overlay should not show native button names")
	}
}

func TestKeybindingsOverlay_GamepadShowsNativeButtons(t *testing.T) {
	loc := i18n.New("en")
	overlay := newKeybindingsOverlay(loc)
	overlay.SetHasGamepad(true)
	overlay.visible = true
	lines := overlay.allLines()
	joined := strings.Join(lines, "\n")
	if strings.Contains(joined, "F13") {
		t.Errorf("with gamepad, overlay should not show F13 keys; got:\n%s", joined)
	}
	if !strings.Contains(joined, "A / Cross") {
		t.Errorf("with gamepad, overlay should show native button names")
	}
	if !strings.Contains(joined, "LB / L1") {
		t.Errorf("with gamepad, overlay should show section navigation buttons")
	}
}

func TestSwitchSection_SetsCompatLine(t *testing.T) {
	course := lesson.Course{
		ID:    "two-section",
		Title: "Two Sections",
		Sections: []lesson.Section{
			{ID: "lessons", Type: "exercises", Lessons: []lesson.Lesson{{ID: "l1", Title: "L1"}}},
			{ID: "quizzes", Type: "quizzes"},
		},
	}
	m := NewMenuScreen([]lesson.Course{course}, nil, nil, nil, nil, "alice", 0, false, func(string) bool {
		return true
	}, testLocale(), 80, 24)
	model, _ := m.enterCourse(course)
	menu := model.(*MenuScreen)

	// Clear any compat line from entering the course.
	menu.compatLine = ""

	menu.switchSection(1) // advance to quizzes
	if menu.compatLine == "" {
		t.Error("switchSection should set compatLine for known section type")
	}
	quizLine := i18n.New("en").T("mochi.section_quizzes")
	if menu.compatLine != quizLine {
		t.Errorf("compatLine = %q, want %q", menu.compatLine, quizLine)
	}
}
