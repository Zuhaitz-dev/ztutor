package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"

	tea "github.com/charmbracelet/bubbletea"
)

// ---------------------------------------------------------------------------
// LessonScreen read-only completion flow
// ---------------------------------------------------------------------------

func TestLessonScreen_Init_ReadOnly_FirstVisitDoesNotAutoComplete(t *testing.T) {
	l := lesson.Lesson{
		ID:      "00-module-intro",
		Content: "# Intro\n\nSome content.",
		// No Exercise, no Files → IsReadOnly() == true
	}
	ls := NewLessonScreen(l, 0, 80, 24, i18n.New("en"))

	cmd := ls.Init()
	if cmd != nil {
		t.Fatal("Init() should not auto-complete a read-only lesson on first visit")
	}
}

func TestLessonScreen_Back_ReadOnly_FirstVisitCompletes(t *testing.T) {
	l := lesson.Lesson{
		ID:      "00-module-intro",
		Content: "# Intro\n\nSome content.",
	}
	ls := NewLessonScreen(l, 0, 80, 24, i18n.New("en"))

	model, cmd := ls.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := model.(*LessonScreen); !ok {
		t.Fatalf("Update returned %T, want *LessonScreen", model)
	}
	if cmd == nil {
		t.Fatal("back on first-visit read-only lesson should emit completion")
	}

	msg := cmd()
	completed, ok := msg.(lessonCompletedMsg)
	if !ok {
		t.Fatalf("back command returned %T, want lessonCompletedMsg", msg)
	}
	if completed.lessonID != "00-module-intro" {
		t.Errorf("lessonID = %q, want 00-module-intro", completed.lessonID)
	}
	if completed.stars != 1 {
		t.Errorf("stars = %d, want 1", completed.stars)
	}
}

func TestLessonScreen_Back_ReadOnly_RevisitReturnsToCourse(t *testing.T) {
	l := lesson.Lesson{
		ID:      "00-module-intro",
		Content: "# Intro\n\nSome content.",
	}
	ls := NewLessonScreen(l, 1, 80, 24, i18n.New("en"))

	_, cmd := ls.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd == nil {
		t.Fatal("back on revisited read-only lesson should navigate back")
	}
	if _, ok := cmd().(NavigateBackToCourse); !ok {
		t.Fatalf("back command returned %T, want NavigateBackToCourse", cmd())
	}
}

func TestLessonScreen_Init_WithExercise(t *testing.T) {
	l := lesson.Lesson{
		ID:       "01-hello",
		Content:  "# Hello\n\nContent.",
		Exercise: "int main(){}",
	}
	ls := NewLessonScreen(l, 0, 80, 24, i18n.New("en"))

	cmd := ls.Init()
	if cmd != nil {
		t.Error("Init() should return nil for a lesson with an exercise")
	}
}

func TestLessonView_RTLHeaderMirrorsInlineOrder(t *testing.T) {
	ls := NewLessonScreen(lesson.Lesson{
		Title:      "Pointers",
		IsPremium:  true,
		Companies:  []string{"Acme"},
		Difficulty: "beginner",
		Tags:       []string{"memory", "arrays"},
		Content:    "Body",
	}, 0, 80, 24, i18n.New("ar"))

	view := stripANSI(ls.View())
	lines := strings.Split(view, "\n")
	if len(lines) < 2 {
		t.Fatalf("lesson view too short: %q", view)
	}
	header := lines[0]
	meta := lines[1]
	badgeIdx := strings.Index(header, "مميز")
	if badgeIdx < 0 {
		badgeIdx = strings.Index(header, "[")
	}
	if !(strings.Index(header, "Acme") < badgeIdx &&
		badgeIdx < strings.Index(header, "Pointers")) {
		t.Fatalf("rtl header should place title last in inline order, got %q", header)
	}
	if !(strings.Index(meta, "memory  arrays") < strings.Index(meta, "beginner")) {
		t.Fatalf("rtl metadata should place difficulty last in inline order, got %q", meta)
	}
}
