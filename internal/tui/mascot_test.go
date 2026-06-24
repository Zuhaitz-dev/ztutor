package tui

import (
	"strings"
	"testing"
)

func TestRenderMascotPanel_TooNarrow(t *testing.T) {
	for _, w := range []int{0, 1, 29} {
		if got := renderMascotPanel(w, "Mochi", "hello", MoodIdle, 0, false); got != "" {
			t.Errorf("width=%d: expected empty string for narrow terminal, got %q", w, got)
		}
	}
}

func TestRenderMascotPanel_LTR(t *testing.T) {
	result := renderMascotPanel(80, "Mochi", "Keep going!", MoodIdle, 0, false)
	if result == "" {
		t.Fatal("expected non-empty result")
	}
	plain := stripANSI(result)
	if !strings.Contains(plain, "Mochi") {
		t.Errorf("expected speaker name in output, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Keep going!") {
		t.Errorf("expected message text in output, got:\n%s", plain)
	}
}

func TestRenderMascotPanel_RTL(t *testing.T) {
	result := renderMascotPanel(80, "Mochi", "Keep going!", MoodIdle, 0, true)
	if result == "" {
		t.Fatal("expected non-empty result for RTL panel")
	}
	plain := stripANSI(result)
	if !strings.Contains(plain, "Mochi") {
		t.Errorf("RTL: expected speaker name in output, got:\n%s", plain)
	}
	if !strings.Contains(plain, "Keep going!") {
		t.Errorf("RTL: expected message text in output, got:\n%s", plain)
	}
}

func TestRenderMascotPanel_DefaultSpeaker(t *testing.T) {
	result := renderMascotPanel(80, "", "", MoodIdle, 0, false)
	plain := stripANSI(result)
	if !strings.Contains(plain, "Mochi") {
		t.Errorf("expected default speaker 'Mochi', got:\n%s", plain)
	}
}

func TestRenderMascotPanel_AllMoods(t *testing.T) {
	moods := []MascotMood{MoodIdle, MoodCurious, MoodHappy, MoodWorried, MoodCrashed, MoodThinking, MoodFocused, MascotMood("unknown")}
	for _, mood := range moods {
		for _, frame := range []int{0, 1, 2, 3} {
			got := renderMascotPanel(80, "Mochi", "test", mood, frame, false)
			if got == "" {
				t.Errorf("mood=%q frame=%d: got empty string", mood, frame)
			}
		}
	}
}

func TestRenderMascotPanel_LTRvRTL_DifferentLayout(t *testing.T) {
	ltr := renderMascotPanel(80, "Mochi", "hello world", MoodIdle, 0, false)
	rtl := renderMascotPanel(80, "Mochi", "hello world", MoodIdle, 0, true)
	if ltr == rtl {
		t.Error("LTR and RTL panels should produce different layouts")
	}
}

func TestWrapPlainText_Basic(t *testing.T) {
	lines := wrapPlainText("hello world foo bar", 10)
	for _, l := range lines {
		if len(l) > 10 {
			t.Errorf("line %q exceeds width 10", l)
		}
	}
}

func TestWrapPlainText_Empty(t *testing.T) {
	if got := wrapPlainText("", 20); got != nil {
		t.Errorf("empty input: got %v, want nil", got)
	}
}

func TestWrapPlainText_ZeroWidth(t *testing.T) {
	got := wrapPlainText("hello", 0)
	if len(got) != 1 || got[0] != "hello" {
		t.Errorf("zero width: got %v, want [hello]", got)
	}
}

func TestWrapPlainText_SingleLongWord(t *testing.T) {
	word := "abcdefghijklmnopqrstuvwxyz"
	lines := wrapPlainText(word, 5)
	if len(lines) == 0 {
		t.Fatal("expected at least one line")
	}
}

func TestWrapPlainText_ExactFit(t *testing.T) {
	lines := wrapPlainText("hello", 5)
	if len(lines) != 1 || lines[0] != "hello" {
		t.Errorf("exact fit: got %v, want [hello]", lines)
	}
}
