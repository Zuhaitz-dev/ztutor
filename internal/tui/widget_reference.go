package tui

import (
	"strings"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type ReferenceWidget struct {
	refs    []string
	visible bool
	loc     *i18n.Locale
}

func newReferenceWidget(refs []string, loc *i18n.Locale) *ReferenceWidget {
	return &ReferenceWidget{refs: refs, loc: loc}
}

func (w *ReferenceWidget) Available() bool { return len(w.refs) > 0 }

// SetReferences replaces the reference list, e.g. after a locale switch.
func (w *ReferenceWidget) SetReferences(refs []string) {
	w.refs = refs
}

func (w *ReferenceWidget) Toggle() { w.visible = !w.visible }

func (w *ReferenceWidget) Hide() { w.visible = false }

func (w *ReferenceWidget) IsVisible() bool { return w.visible }

func (w *ReferenceWidget) View() string {
	if !w.visible || len(w.refs) == 0 {
		return ""
	}
	headerStyle := te.NewStyle().Bold(true).Foreground(te.Color("117"))
	urlStyle := te.NewStyle().Foreground(te.Color("39"))
	bookStyle := te.NewStyle().Foreground(te.Color("214"))
	manStyle := te.NewStyle().Foreground(te.Color("252"))

	var b strings.Builder
	b.WriteString(headerStyle.Render(w.loc.T("exercise.reference.header")))
	b.WriteString("\n")

	for _, ref := range w.refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		var styled string
		switch {
		case strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://"):
			styled = "  " + urlStyle.Render("→ ") + urlStyle.Render(ref)
		case strings.HasPrefix(ref, "man "):
			styled = "  " + manStyle.Render("man ") + manStyle.Render(strings.TrimPrefix(ref, "man "))
		default:
			styled = "  " + bookStyle.Render("[ref] ") + bookStyle.Render(ref)
		}
		b.WriteString(styled)
		b.WriteString("\n")
	}
	return b.String()
}
