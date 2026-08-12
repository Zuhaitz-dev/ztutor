package tui

import (
	"strings"

	"ztutor/internal/i18n"

	"github.com/charmbracelet/lipgloss"
)

func starsStyle(stars, maxStars int) string {
	if maxStars == 1 {
		if stars >= 1 {
			return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("★  ")
		}
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○  ")
	}
	switch stars {
	case 3:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render("★★★")
	case 2:
		return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber)).Render("★★☆")
	case 1:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Render("★☆☆")
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("○  ")
	}
}

// renderKey is the single source of truth for how a keyboard shortcut looks
// anywhere in the TUI. Both help bars and the keybindings overlay call this.
func renderKey(k string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(ColorBody)).
		Background(lipgloss.Color(ColorBorder)).
		Bold(true).
		Padding(0, 1).
		Render(k)
}

func renderHelpItem(key, label string) string {
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	if strings.TrimSpace(key) == "" {
		return descStyle.Render(label)
	}
	if strings.TrimSpace(label) == "" {
		return renderKey(key)
	}
	return renderKey(key) + " " + descStyle.Render(label)
}

func actionHelpBar(loc *i18n.Locale, actions ...HelpAction) string {
	parts := make([]string, 0, len(actions))
	for _, item := range actions {
		action, ok := LookupKeyAction(item.ID)
		if !ok {
			continue
		}
		parts = append(parts, renderHelpItem(keyDisplay(action.Keys), actionLabel(loc, action, item.Args...)))
	}
	return strings.Join(parts, "  ")
}

func helpBar(items ...string) string {
	var parts []string
	for _, item := range items {
		idx := strings.Index(item, " ")
		if idx > 0 {
			parts = append(parts, renderHelpItem(item[:idx], item[idx+1:]))
		} else {
			parts = append(parts, renderHelpItem("", item))
		}
	}
	return strings.Join(parts, "  ")
}

func bold(s string) string {
	return lipgloss.NewStyle().Bold(true).Render(s)
}

func joinInlineParts(rtl bool, parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		if strings.TrimSpace(part) != "" {
			filtered = append(filtered, part)
		}
	}
	if rtl {
		for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
			filtered[i], filtered[j] = filtered[j], filtered[i]
		}
	}
	return strings.Join(filtered, "  ")
}

// rtlAlignBlock right-aligns every line of s to width when the caller is in
// an RTL locale. Each line is padded independently so ANSI styles are preserved.
func rtlAlignBlock(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = lipgloss.NewStyle().Width(width).Align(lipgloss.Right).Render(line)
	}
	return strings.Join(lines, "\n")
}

func dim(s string) string {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(ColorFaded)).Render(s)
}

func titleStyle(s string) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(ColorAccent)).
		Render(s)
}

func codeStyle(s string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Render(s)
}
