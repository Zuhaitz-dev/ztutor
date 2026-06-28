package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/i18n"
	"ztutor/internal/license"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type licenseSummaryScreen struct {
	loc *i18n.Locale
	lic *license.State
	sized
}

func NewLicenseSummaryScreen(loc *i18n.Locale, lic *license.State, w, h int) *licenseSummaryScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	return &licenseSummaryScreen{loc: loc, lic: lic, sized: sized{Width: w, Height: h}}
}

func (s *licenseSummaryScreen) Init() tea.Cmd { return nil }

func (s *licenseSummaryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.HandleResize(msg)
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return s, backCmd(NavigateToMenu{})
		case "esc":
			return s, backCmd(NavigateToConnectChoice{})
		case "q", "ctrl+c":
			return s, tea.Quit
		}
	}
	return s, nil
}

func productTierLabel(loc *i18n.Locale, lic *license.State) string {
	if loc == nil {
		loc = i18n.New("en")
	}
	key := "license_summary.tier." + lic.ProductTier()
	return loc.T(key)
}

func localizedLicenseFeatures(loc *i18n.Locale, lic *license.State) string {
	if lic == nil || !lic.Licensed {
		return loc.T("license_summary.features_none")
	}
	var feats []string
	if lic.HasMultiUser {
		feats = append(feats, loc.T("license_summary.feature.multi_user"))
	}
	if lic.HasAdminUI {
		feats = append(feats, loc.T("license_summary.feature.admin_ui"))
	}
	if lic.HasInterviewQuestions {
		feats = append(feats, loc.T("license_summary.feature.interviews"))
	}
	if len(feats) == 0 {
		return loc.T("license_summary.features_none")
	}
	return strings.Join(feats, ", ")
}

func localizedLicenseCourseAccess(loc *i18n.Locale, lic *license.State) string {
	if lic == nil || !lic.Licensed {
		return loc.T("license_summary.courses_free")
	}
	for _, id := range lic.UnlockedCourses {
		if id == "*" {
			return loc.T("license_summary.courses_all")
		}
	}
	return loc.T("license_summary.courses_count", len(lic.UnlockedCourses))
}

func (s *licenseSummaryScreen) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	border := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorBorder)).
		Padding(1, 2).
		Width(min(64, max(44, s.Width-8)))

	lines := []string{
		titleStyle.Render(s.loc.T("license_summary.title")),
		"",
		labelStyle.Render(s.loc.T("license_summary.tier")) + " " + valueStyle.Render(productTierLabel(s.loc, s.lic)),
		labelStyle.Render(s.loc.T("license_summary.licensee")) + " " + valueStyle.Render(forceLTRText(s.lic.Licensee)),
		labelStyle.Render(s.loc.T("license_summary.courses")) + " " + valueStyle.Render(localizedLicenseCourseAccess(s.loc, s.lic)),
	}
	if s.lic.MaxStudents > 0 {
		lines = append(lines, labelStyle.Render(s.loc.T("license_summary.seats"))+" "+valueStyle.Render(fmt.Sprintf("%d", s.lic.MaxStudents)))
	}
	lines = append(lines, labelStyle.Render(s.loc.T("license_summary.features"))+" "+valueStyle.Render(localizedLicenseFeatures(s.loc, s.lic)))
	lines = append(lines, "", helpStyle.Render(s.loc.T("license_summary.help")))

	content := strings.Join(lines, "\n")
	return center(s.Width, s.Height, border.Render(content))
}
