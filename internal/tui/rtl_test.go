package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestRtlAlignBlock_MultiLine(t *testing.T) {
	input := "hello\nworld\nfoo"
	got := rtlAlignBlock(input, 20)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("line count = %d, want 3", len(lines))
	}
	for i, line := range lines {
		w := lipgloss.Width(line)
		if w != 20 {
			t.Errorf("line %d: visual width = %d, want 20", i, w)
		}
	}
}

func TestRtlAlignBlock_EmptyInput(t *testing.T) {
	got := rtlAlignBlock("", 20)
	// Should not panic; result is a single right-aligned empty line of width 20.
	if lipgloss.Width(got) > 20 {
		t.Errorf("width %d exceeds requested 20", lipgloss.Width(got))
	}
}

func TestRtlAlignBlock_ZeroWidth(t *testing.T) {
	got := rtlAlignBlock("hello", 0)
	if got != "hello" {
		t.Errorf("zero width: got %q, want %q", got, "hello")
	}
}

func TestRtlAlignBlock_NegativeWidth(t *testing.T) {
	got := rtlAlignBlock("hello", -1)
	if got != "hello" {
		t.Errorf("negative width: got %q, want %q", got, "hello")
	}
}

func TestPadVis_NoPadNeeded(t *testing.T) {
	got := padVis("hello", 5)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestPadVis_PadsRight(t *testing.T) {
	got := padVis("hi", 5)
	if lipgloss.Width(got) != 5 {
		t.Errorf("width = %d, want 5", lipgloss.Width(got))
	}
	if !strings.HasPrefix(got, "hi") {
		t.Errorf("expected prefix 'hi', got %q", got)
	}
}

func TestPadVis_AlreadyWider(t *testing.T) {
	got := padVis("hello world", 5)
	if got != "hello world" {
		t.Errorf("wider input should pass through unchanged, got %q", got)
	}
}

func TestTruncRunes_ShortInput(t *testing.T) {
	got := truncRunes("hello", 10)
	if got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestTruncRunes_LongInput(t *testing.T) {
	got := truncRunes("hello world", 6)
	runes := []rune(got)
	if len(runes) > 6 {
		t.Errorf("result %q too long (%d runes), want <=6", got, len(runes))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected ellipsis suffix, got %q", got)
	}
}

func TestTruncRunes_ZeroWidth(t *testing.T) {
	got := truncRunes("hello", 0)
	if got != "" {
		t.Errorf("zero width: got %q, want empty", got)
	}
}
