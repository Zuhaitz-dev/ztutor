package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NavigateToAchievements struct{}

type AchievementScreen struct {
	earned map[string]bool
	sized
	loc *i18n.Locale
}

func NewAchievementScreen(earnedIDs []string, width, height int, loc *i18n.Locale) *AchievementScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	earned := make(map[string]bool, len(earnedIDs))
	for _, id := range earnedIDs {
		earned[id] = true
	}
	return &AchievementScreen{earned: earned, sized: sized{Width: width, Height: height}, loc: loc}
}

func (as *AchievementScreen) Init() tea.Cmd { return nil }

func (as *AchievementScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case KeyBack, KeyBackAlt, KeyBackEditor:
			return as, backCmd(NavigateToMenu{})
		}
	case tea.WindowSizeMsg:
		as.HandleResize(msg)
	}
	return as, nil
}

func (as *AchievementScreen) View() string {
	var b strings.Builder

	// Secret achievements are invisible until earned; exclude them from totals.
	earnedCount, visibleTotal := 0, 0
	for _, a := range allAchievements {
		isEarned := as.earned[a.ID]
		if a.Secret && !isEarned {
			continue
		}
		visibleTotal++
		if isEarned {
			earnedCount++
		}
	}

	T := as.loc.T
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	b.WriteString(titleStyle.Render(T("achievements.title")))
	b.WriteString("  ")
	b.WriteString(dim(T("achievements.earned", earnedCount, visibleTotal)))
	b.WriteString("\n\n")

	iconEarned := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true)
	iconLocked := lipgloss.NewStyle().Foreground(lipgloss.Color("238"))
	nameEarned := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	nameLocked := lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242"))
	earnedBadge := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render("earned")

	for _, a := range allAchievements {
		isEarned := as.earned[a.ID]
		if a.Secret && !isEarned {
			continue // stay hidden
		}

		if isEarned {
			b.WriteString("  ")
			b.WriteString(iconEarned.Render(fmt.Sprintf("%-4s", a.Icon)))
			b.WriteString("  ")
			b.WriteString(nameEarned.Render(fmt.Sprintf("%-16s", a.Name)))
			b.WriteString("  ")
			b.WriteString(descStyle.Render(a.Desc))
			b.WriteString("  ")
			b.WriteString(earnedBadge)
		} else {
			b.WriteString("  ")
			b.WriteString(iconLocked.Render(fmt.Sprintf("%-4s", a.Icon)))
			b.WriteString("  ")
			b.WriteString(nameLocked.Render(fmt.Sprintf("%-16s", a.Name)))
			b.WriteString("  ")
			b.WriteString(nameLocked.Render(a.Desc))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpBar(T("help.q_back")))
	result := b.String()
	return rtlWrap(as.loc.IsRTL(), result, as.Width)
}
