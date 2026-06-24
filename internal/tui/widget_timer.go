package tui

import (
	"fmt"
	"time"

	te "github.com/charmbracelet/lipgloss"
)

type TimerWidget struct {
	start   time.Time
	running bool
	elapsed time.Duration
	visible bool
}

func newTimerWidget() *TimerWidget {
	return &TimerWidget{}
}

func (w *TimerWidget) Available() bool { return true }

func (w *TimerWidget) Start() {
	w.start = time.Now()
	w.running = true
	w.visible = true
}

func (w *TimerWidget) Stop() {
	if w.running {
		w.elapsed = time.Since(w.start)
		w.running = false
	}
}

func (w *TimerWidget) Reset() {
	w.running = false
	w.elapsed = 0
	w.visible = false
}

func (w *TimerWidget) Toggle() { w.visible = !w.visible }

func (w *TimerWidget) IsVisible() bool { return w.visible }

func (w *TimerWidget) Current() time.Duration {
	if w.running {
		return time.Since(w.start)
	}
	return w.elapsed
}

func (w *TimerWidget) View() string {
	d := w.Current()
	if d == 0 && !w.running {
		return ""
	}
	if !w.visible {
		return ""
	}
	sec := d.Seconds()
	var color string
	switch {
	case sec < 5:
		color = "42"
	case sec < 15:
		color = "220"
	case sec < 30:
		color = "214"
	default:
		color = "196"
	}
	timerStyle := te.NewStyle().Foreground(te.Color(color)).Bold(true)
	label := "⏱ "
	if w.running {
		label = "▶ "
	}
	return timerStyle.Render(fmt.Sprintf("%s%.1fs", label, sec))
}
