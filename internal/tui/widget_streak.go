package tui

import (
	"fmt"

	"ztutor/internal/i18n"

	te "github.com/charmbracelet/lipgloss"
)

type StreakWidget struct {
	streak  int
	visible bool
	loc     *i18n.Locale
}

func newStreakWidget(streak int, loc *i18n.Locale) *StreakWidget {
	return &StreakWidget{streak: streak, visible: streak > 0, loc: loc}
}

func (w *StreakWidget) Available() bool { return w.streak > 0 }
func (w *StreakWidget) IsVisible() bool { return w.visible }
func (w *StreakWidget) Hide()           { w.visible = false }
func (w *StreakWidget) SetStreak(v int) {
	w.streak = v
	w.visible = v > 0
}

func (w *StreakWidget) View() string {
	if !w.visible || w.streak <= 0 {
		return ""
	}
	streakStyle := te.NewStyle().Foreground(te.Color("214")).Bold(true)
	return streakStyle.Render(fmt.Sprintf("%s %d %s",
		w.loc.T("streak.label"), w.streak, w.loc.T("streak.days")))
}
