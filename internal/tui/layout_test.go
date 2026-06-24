package tui

import (
	"strings"
	"testing"
)

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

func TestLayout_FixedOnly(t *testing.T) {
	l := NewTerminalLayout(80, 25)
	l.AddFixed("a", nil, func(w int) string { return "line a1\nline a2" })
	l.AddFixed("b", nil, func(w int) string { return "line b1" })

	out := l.Render()
	lines := countLines(out)
	if lines < 3 {
		t.Errorf("expected >= 3 lines, got %d\n%s", lines, out)
	}
	if lines != 25 {
		t.Errorf("expected exactly 25 lines (padded to terminal), got %d", lines)
	}
	if !strings.Contains(out, "line a1") || !strings.Contains(out, "line b1") {
		t.Errorf("missing expected content:\n%s", out)
	}
}

func TestLayout_FixedAndFlex(t *testing.T) {
	l := NewTerminalLayout(80, 30)
	l.AddFixed("header", nil, func(w int) string { return "header line" })
	l.AddFlex("editor", 1, nil, func(w, h int) string {
		return strings.Repeat("editor line\n", h-1) + "editor last"
	})
	l.AddFixed("footer", nil, func(w int) string { return "footer line" })

	out := l.Render()
	lines := countLines(out)
	if lines != 30 {
		t.Errorf("expected 30 lines, got %d\n%s", lines, out)
	}
	if !strings.Contains(out, "header line") {
		t.Errorf("missing header")
	}
	if !strings.Contains(out, "editor line") {
		t.Errorf("missing editor lines")
	}
	if !strings.Contains(out, "footer line") {
		t.Errorf("missing footer")
	}
}

func TestLayout_TwoFlex(t *testing.T) {
	l := NewTerminalLayout(80, 40)
	l.AddFixed("top", nil, func(w int) string { return "top" })
	l.AddFlex("editor", 1, nil, func(w, h int) string {
		return strings.Repeat("e\n", h-1) + "e"
	})
	l.AddFlex("output", 1, nil, func(w, h int) string {
		return strings.Repeat("o\n", h-1) + "o"
	})
	l.AddFixed("bot", nil, func(w int) string { return "bottom" })

	out := l.Render()
	lines := countLines(out)
	if lines != 40 {
		t.Errorf("expected 40 lines, got %d", lines)
	}
	eCount := strings.Count(out, "e ")
	oCount := strings.Count(out, "o ")
	if eCount < 5 || oCount < 5 {
		t.Errorf("flex slots share remaining height roughly equally, e=%d o=%d", eCount, oCount)
	}
}

func TestLayout_HiddenSlot(t *testing.T) {
	visible := true
	l := NewTerminalLayout(80, 20)
	l.AddFixed("header", nil, func(w int) string { return "header" })
	l.AddFixed("opt", func() bool { return visible }, func(w int) string {
		return "optional\nsecond"
	})
	l.AddFlex("main", 1, nil, func(w, h int) string {
		return strings.Repeat("x\n", h-1) + "x"
	})

	out1 := l.Render()
	if !strings.Contains(out1, "optional") {
		t.Errorf("visible=true should include optional slot")
	}

	visible = false
	out2 := l.Render()
	if strings.Contains(out2, "optional") {
		t.Errorf("visible=false should hide optional slot")
	}
	if countLines(out2) != 20 {
		t.Errorf("expected 20 lines after hiding, got %d", countLines(out2))
	}
}

func TestLayout_AllFlex(t *testing.T) {
	l := NewTerminalLayout(80, 30)
	l.AddFlex("a", 2, nil, func(w, h int) string {
		return strings.Repeat("A\n", h-1) + "A"
	})
	l.AddFlex("b", 1, nil, func(w, h int) string {
		return strings.Repeat("B\n", h-1) + "B"
	})

	out := l.Render()
	lines := countLines(out)
	if lines != 30 {
		t.Errorf("expected 30 lines, got %d", lines)
	}
	aCount := strings.Count(out, "\n")
	bCount := strings.Count(out, "B")
	t.Logf("A newlines=%d, B count=%d, total lines=%d", aCount, bCount, lines)
}

func TestLayout_NegativeRemaining(t *testing.T) {
	l := NewTerminalLayout(80, 5)
	l.AddFixed("big1", nil, func(w int) string {
		return "line1\nline2\nline3\nline4"
	})
	l.AddFlex("small", 1, nil, func(w, h int) string {
		return strings.Repeat("s\n", h-1) + "s"
	})

	out := l.Render()
	lines := countLines(out)
	if lines != 5 {
		t.Errorf("expected exactly 5 lines (terminal height), got %d", lines)
	}
}

func TestLayout_ZeroHeight(t *testing.T) {
	l := NewTerminalLayout(80, 3)
	l.AddFlex("only", 1, nil, func(w, h int) string {
		return strings.Repeat("x\n", h-1) + "x"
	})
	out := l.Render()
	if countLines(out) != 3 {
		t.Errorf("expected 3 lines, got %d", countLines(out))
	}
}

func TestLayout_EmptyFixed(t *testing.T) {
	l := NewTerminalLayout(80, 10)
	l.AddFixed("empty", nil, func(w int) string { return "" })
	l.AddFlex("main", 1, nil, func(w, h int) string {
		return strings.Repeat("m\n", h-1) + "m"
	})
	out := l.Render()
	if countLines(out) != 10 {
		t.Errorf("empty fixed should take 0 lines, expected 10, got %d", countLines(out))
	}
}
