package tui

import (
	"strings"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
)

func TestLessonView_RTLHeaderMirrorsInlineOrder(t *testing.T) {
	ls := NewLessonScreen(lesson.Lesson{
		Title:      "Pointers",
		IsPremium:  true,
		Companies:  []string{"Acme"},
		Difficulty: "beginner",
		Tags:       []string{"memory", "arrays"},
		Content:    "Body",
	}, 80, 24, i18n.New("ar"))

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
