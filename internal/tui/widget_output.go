package tui

import "strings"

// OutputWidget holds the output panel content and scroll state.
// It is not in the Tab cycle — it is managed directly by ExerciseScreen.
type OutputWidget struct {
	content string
	offset  int
}

func newOutputWidget() *OutputWidget {
	return &OutputWidget{}
}

// SetContent replaces the panel content and resets the scroll position.
func (w *OutputWidget) SetContent(s string) {
	w.content = s
	w.offset = 0
}

// Content returns the raw content string.
func (w *OutputWidget) Content() string {
	return w.content
}

// ScrollDown moves the view down by one line, clamped to the visible range.
func (w *OutputWidget) ScrollDown(height int) {
	lines := strings.Split(w.content, "\n")
	max := len(lines) - height
	if max < 0 {
		max = 0
	}
	if w.offset < max {
		w.offset++
	}
}

// ScrollUp moves the view up by one line.
func (w *OutputWidget) ScrollUp() {
	if w.offset > 0 {
		w.offset--
	}
}

// ScrollTop resets the view to the top.
func (w *OutputWidget) ScrollTop() {
	w.offset = 0
}

// ScrollBottom jumps the view to the last screenful of content.
func (w *OutputWidget) ScrollBottom(height int) {
	lines := strings.Split(w.content, "\n")
	max := len(lines) - height
	if max < 0 {
		max = 0
	}
	w.offset = max
}

// ViewScrolled returns the visible slice of content starting at the current
// scroll offset. If the panel has not been scrolled, returns the full content.
func (w *OutputWidget) ViewScrolled(height int) string {
	if w.offset == 0 {
		return w.content
	}
	lines := strings.Split(w.content, "\n")
	start := w.offset
	if start >= len(lines) {
		start = len(lines) - 1
	}
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}
