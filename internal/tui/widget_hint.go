package tui

import (
	"ztutor/internal/i18n"

	"github.com/charmbracelet/lipgloss"
)

// HintWidget manages hint cycling and visibility for an exercise.
// It is not in the Tab cycle — it is driven directly by ExerciseScreen.
type HintWidget struct {
	hints   []string
	index   int // -1 means no hint shown yet
	visible bool
	used    int // count of distinct hints revealed this session
	loc     *i18n.Locale
}

func newHintWidget(hints []string, loc *i18n.Locale) *HintWidget {
	return &HintWidget{hints: hints, index: -1, loc: loc}
}

// Available reports whether this exercise has any hints.
func (w *HintWidget) Available() bool { return len(w.hints) > 0 }

// SetLocale updates the locale used for rendering hint headers.
func (w *HintWidget) SetLocale(loc *i18n.Locale) { w.loc = loc }

// Next advances to the next hint and makes it visible.
// Wraps around after the last hint. Tracks how many distinct hints were used.
func (w *HintWidget) Next() {
	if len(w.hints) == 0 {
		return
	}
	w.index = (w.index + 1) % len(w.hints)
	w.visible = true
	if w.index+1 > w.used {
		w.used = w.index + 1
	}
}

// Hide hides the hint overlay without resetting which hint was last shown.
func (w *HintWidget) Hide() { w.visible = false }

// IsVisible reports whether the hint overlay should be shown.
func (w *HintWidget) IsVisible() bool { return w.visible && w.index >= 0 && w.index < len(w.hints) }

// HintsUsed returns the count of distinct hints revealed this session.
func (w *HintWidget) HintsUsed() int { return w.used }

// CurrentIndex returns the 0-based index of the last shown hint, or -1 if none.
func (w *HintWidget) CurrentIndex() int { return w.index }

// View renders the hint overlay content: a header line followed by the hint body.
// Call only when IsVisible() returns true.
func (w *HintWidget) View() string {
	if !w.IsVisible() {
		return ""
	}
	T := w.loc.T
	headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorHex))
	bodyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	header := headerStyle.Render(T("exercise.hint_header", w.index+1, len(w.hints)))
	return header + "\n\n" + bodyStyle.Render(w.hints[w.index])
}
