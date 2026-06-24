package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NavigateToLeaderboard struct{}

type LeaderboardScreen struct {
	entries      []db.LeaderboardEntry
	self         string
	totalLessons int
	sized
	loc *i18n.Locale
}

func NewLeaderboardScreen(entries []db.LeaderboardEntry, self string, totalLessons, width, height int, loc *i18n.Locale) *LeaderboardScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	return &LeaderboardScreen{
		entries:      entries,
		self:         self,
		totalLessons: totalLessons,
		sized:        sized{Width: width, Height: height},
		loc:          loc,
	}
}

func (ls *LeaderboardScreen) Init() tea.Cmd { return nil }

func (ls *LeaderboardScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case KeyBack, KeyBackAlt, KeyBackEditor:
			return ls, backCmd(NavigateToMenu{})
		}
	case tea.WindowSizeMsg:
		ls.HandleResize(msg)
	}
	return ls, nil
}

func (ls *LeaderboardScreen) View() string {
	var b strings.Builder

	T := ls.loc.T
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	b.WriteString(titleStyle.Render(T("leaderboard.title")))
	b.WriteString("\n\n")

	if len(ls.entries) == 0 {
		b.WriteString(dim(T("leaderboard.empty")))
		b.WriteString("\n\n")
		b.WriteString(helpBar(T("help.q_back")))
		result := b.String()
		return rtlWrap(ls.loc.IsRTL(), result, ls.Width)
	}

	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim)).Bold(true)
	b.WriteString(headerStyle.Render(fmt.Sprintf("  %-4s  %-20s  %5s  %s", T("leaderboard.col_rank"), T("leaderboard.col_user"), T("leaderboard.col_stars"), T("leaderboard.col_progress"))))
	b.WriteString("\n")
	b.WriteString(dim(strings.Repeat("─", min(ls.Width-2, 52))))
	b.WriteString("\n")

	goldStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true)
	silverStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("250"))
	bronzeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("172"))
	selfStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAccent)).Bold(true)
	plainStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))

	for i, e := range ls.entries {
		rank := i + 1
		isSelf := e.Username == ls.self

		username := e.Username
		if len(username) > 20 {
			username = username[:17] + "..."
		}
		if isSelf {
			username += " " + T("leaderboard.self_label")
		}

		rankMedal := fmt.Sprintf("%d", rank)
		switch rank {
		case 1:
			rankMedal = "1st"
		case 2:
			rankMedal = "2nd"
		case 3:
			rankMedal = "3rd"
		}

		progress := fmt.Sprintf("%d/%d", e.LessonsDone, ls.totalLessons)
		line := fmt.Sprintf("  %-4s  %-20s  %5d  %s", rankMedal, username, e.TotalStars, progress)

		var rowStyle lipgloss.Style
		switch {
		case isSelf:
			rowStyle = selfStyle
		case rank == 1:
			rowStyle = goldStyle
		case rank == 2:
			rowStyle = silverStyle
		case rank == 3:
			rowStyle = bronzeStyle
		default:
			rowStyle = plainStyle
		}

		b.WriteString(rowStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpBar(T("help.q_back")))
	result := b.String()
	return rtlWrap(ls.loc.IsRTL(), result, ls.Width)
}
