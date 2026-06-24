package tui

import (
	"fmt"
	"os"

	"ztutor/internal/i18n"
	"ztutor/internal/license"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type licenseEntryScreen struct {
	loc *i18n.Locale
	sized
	form     *simpleForm
	licState *license.State
}

func NewLicenseEntryScreen(loc *i18n.Locale, w, h int) *licenseEntryScreen {
	s := &licenseEntryScreen{
		loc:   loc,
		sized: sized{Width: w, Height: h},
	}
	s.form = newSimpleForm(
		loc.T("license_entry.placeholder"),
		loc.T("license_entry.title"),
		loc.T("license_entry.prompt"),
		loc.T("license_entry.save"),
		s.tryLoadLicense,
		NavigateToConnectChoice{},
	)
	s.form.input.Width = 60
	return s
}

func (s *licenseEntryScreen) Init() tea.Cmd { return textinput.Blink }

func (s *licenseEntryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.HandleResize(msg)
	case licenseEntryDoneMsg:
		s.handleResult(msg)
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return s, tea.Quit
		}
	}
	m, cmd := s.form.Update(msg)
	s.form = m.(*simpleForm)
	return s, cmd
}

func (s *licenseEntryScreen) tryLoadLicense(val string) tea.Cmd {
	return func() tea.Msg {
		path := val
		if _, err := os.Stat(path); os.IsNotExist(err) {
			tmpFile, err := os.CreateTemp("", "ztutor-license-")
			if err != nil {
				return licenseEntryDoneMsg{}
			}
			defer tmpFile.Close()
			if _, err := tmpFile.WriteString(val); err != nil {
				os.Remove(tmpFile.Name())
				return licenseEntryDoneMsg{}
			}
			path = tmpFile.Name()
			defer os.Remove(tmpFile.Name())
		}

		state, _ := license.Check(path)
		return licenseEntryDoneMsg{licState: state}
	}
}

func (s *licenseEntryScreen) handleResult(msg licenseEntryDoneMsg) {
	if msg.licState != nil {
		s.licState = msg.licState
		n := len(msg.licState.UnlockedCourses)
		s.form.SetMessage(fmt.Sprintf(s.loc.T("license_entry.valid"), msg.licState.Licensee, n), false)
	} else {
		s.form.SetMessage(fmt.Sprintf(s.loc.T("license_entry.invalid"), "verification failed"), true)
	}
}

func (s *licenseEntryScreen) View() string {
	return center(s.Width, s.Height, s.form.View())
}
