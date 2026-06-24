package tui

import (
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type ConsoleWidget struct {
	content string
	visible bool
	loc     *i18n.Locale
}

func newConsoleWidget(loc *i18n.Locale) *ConsoleWidget {
	return &ConsoleWidget{loc: loc}
}

func (w *ConsoleWidget) Available() bool { return true }
func (w *ConsoleWidget) IsVisible() bool { return w.visible }

func (w *ConsoleWidget) SetContent(text string) {
	w.content = strings.TrimSpace(text)
	w.visible = w.content != ""
}

func (w *ConsoleWidget) Clear() {
	w.content = ""
	w.visible = false
}

func (w *ConsoleWidget) Toggle() { w.visible = !w.visible }
func (w *ConsoleWidget) Hide()   { w.visible = false }

func (w *ConsoleWidget) View() string {
	if !w.visible || w.content == "" {
		return ""
	}
	headerStyle := te.NewStyle().Bold(true).Foreground(te.Color("196"))
	errorStyle := te.NewStyle().Foreground(te.Color("196"))
	warnStyle := te.NewStyle().Foreground(te.Color("214"))

	var b strings.Builder
	b.WriteString(headerStyle.Render(w.loc.T("exercise.console.header")))
	b.WriteString("\n")

	for _, line := range strings.Split(w.content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "error:"):
			b.WriteString(errorStyle.Render("  " + line))
		case strings.Contains(lower, "warning:"):
			b.WriteString(warnStyle.Render("  " + line))
		default:
			b.WriteString("  " + line)
		}
		b.WriteString("\n")
	}
	return b.String()
}
