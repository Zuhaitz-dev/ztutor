package tui

import (
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type NavigateToSettings struct{}

type settingsSavedMsg struct {
	key   string
	value string
}

// persistSettingMsg silently saves a setting without navigation (used by
// exercise screen toggles like mascot/timer).
type persistSettingMsg struct {
	key   string
	value string
}

type SettingsScreen struct {
	username   string
	db         *db.DB
	keymap     string
	showMascot bool
	showTimer  bool
	cursor     int
	sized
	loc *i18n.Locale
}

var (
	settingsTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorAccent)).
				MarginBottom(1)

	settingsKeyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorBody)).
				Width(20)

	settingsValueStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color(ColorAmber))

	settingsCursorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorAccent))
)

func NewSettingsScreen(username string, database *db.DB, currentKeymap string, showMascot, showTimer bool, width, height int, loc *i18n.Locale) *SettingsScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	return &SettingsScreen{
		username:   username,
		db:         database,
		keymap:     currentKeymap,
		showMascot: showMascot,
		showTimer:  showTimer,
		sized:      sized{Width: width, Height: height},
		loc:        loc,
	}
}

func (s *SettingsScreen) Init() tea.Cmd { return nil }

func (s *SettingsScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.HandleResize(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case KeyBackEditor, KeyBackAlt, KeyBack:
			return s, backCmd(NavigateToMenu{})

		case KeySelect, KeySelectAlt:
			switch s.cursor {
			case 0: // keymap
				if s.keymap == "vim" {
					s.keymap = "default"
				} else {
					s.keymap = "vim"
				}
				return s, backCmd(settingsSavedMsg{key: "keymap", value: s.keymap})
			case 1: // mascot
				s.showMascot = !s.showMascot
				val := "1"
				if !s.showMascot {
					val = "0"
				}
				return s, backCmd(settingsSavedMsg{key: "mascot_visible", value: val})
			case 2: // timer
				s.showTimer = !s.showTimer
				val := "1"
				if !s.showTimer {
					val = "0"
				}
				return s, backCmd(settingsSavedMsg{key: "timer_visible", value: val})
			}

		case KeyUp, KeyUpVim:
			if s.cursor > 0 {
				s.cursor--
			}
		case KeyDown, KeyDownVim:
			if s.cursor < 2 {
				s.cursor++
			}
		}
	}
	return s, nil
}

func (s *SettingsScreen) View() string {
	T := s.loc.T
	var b strings.Builder
	b.WriteString(settingsTitleStyle.Render(T("settings.title")))
	b.WriteString("\n\n")

	items := []struct {
		label string
		value string
		hint  string
	}{
		{
			label: T("settings.keymap_label"),
			value: s.keymap,
			hint:  T("settings.keymap_hint"),
		},
		{
			label: T("settings.mascot_label"),
			value: boolStr(s.showMascot),
			hint:  T("settings.mascot_hint"),
		},
		{
			label: T("settings.timer_label"),
			value: boolStr(s.showTimer),
			hint:  T("settings.timer_hint"),
		},
	}

	for i, item := range items {
		if item.value == "" {
			item.value = "default"
		}
		prefix := "  "
		if i == s.cursor {
			prefix = settingsCursorStyle.Render("❯") + " "
		}
		b.WriteString(prefix)
		b.WriteString(settingsKeyStyle.Render(item.label))
		b.WriteString(settingsValueStyle.Render(item.value))
		b.WriteString(dim("  " + item.hint))
		b.WriteString("\n")
	}

	b.WriteString("\n\n")
	b.WriteString(actionHelpBar(s.loc, HA(ActionToggleSettings), HA(ActionBack)))
	return rtlWrap(s.loc.IsRTL(), b.String(), s.Width)
}

func boolStr(v bool) string {
	if v {
		return "on"
	}
	return "off"
}
