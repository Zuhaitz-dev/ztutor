package tui

import (
	"strings"

	te "github.com/charmbracelet/lipgloss"
)

type ProgressWidget struct {
	total   int
	passed  int
	visible bool
}

func newProgressWidget() *ProgressWidget {
	return &ProgressWidget{}
}

func (w *ProgressWidget) Available() bool { return w.total > 0 }

func (w *ProgressWidget) SetResult(total, passed int) {
	w.total = total
	w.passed = passed
	w.visible = total > 0
}

func (w *ProgressWidget) Hide() { w.visible = false }

func (w *ProgressWidget) IsVisible() bool { return w.visible }

func (w *ProgressWidget) View() string {
	if w.total == 0 {
		return ""
	}
	passStyle := te.NewStyle().Foreground(te.Color("42"))
	failStyle := te.NewStyle().Foreground(te.Color("196"))
	pendingStyle := te.NewStyle().Foreground(te.Color("240"))
	var b strings.Builder
	for i := 1; i <= w.total; i++ {
		if i <= w.passed {
			b.WriteString(passStyle.Render("[✓]"))
		} else if i > w.passed && w.passed < w.total {
			b.WriteString(failStyle.Render("[✗]"))
		} else {
			b.WriteString(pendingStyle.Render("[ ]"))
		}
		if i < w.total {
			b.WriteString(" ")
		}
	}
	return b.String()
}
