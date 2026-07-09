package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"ztutor/internal/i18n"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type licenseEntryScreen struct {
	loc *i18n.Locale
	sized
	form *simpleForm
}

func NewLicenseEntryScreen(loc *i18n.Locale, w, h int, onSubmit func(string) tea.Cmd) *licenseEntryScreen {
	s := &licenseEntryScreen{
		loc:   loc,
		sized: sized{Width: w, Height: h},
	}
	s.form = newSimpleForm(
		loc.T("license_entry.placeholder"),
		loc.T("license_entry.title"),
		loc.T("license_entry.prompt"),
		loc.T("license_entry.save"),
		onSubmit,
		NavigateToConnectChoice{},
	)
	s.form.input.Width = 60
	return s
}

func (s *licenseEntryScreen) Init() tea.Cmd { return textinput.Blink }

func (s *licenseEntryScreen) SetLocale(loc *i18n.Locale) {
	s.loc = loc
	s.form.title = loc.T("license_entry.title")
	s.form.prompt = loc.T("license_entry.prompt")
	s.form.help = loc.T("license_entry.save")
	s.form.input.Placeholder = loc.T("license_entry.placeholder")
	s.form.SetMessage("", false)
}

func (s *licenseEntryScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.HandleResize(msg)
	case licenseEntryDoneMsg:
		if msg.err == nil && msg.licState != nil {
			n := len(msg.licState.UnlockedCourses)
			s.form.SetMessage(fmt.Sprintf(s.loc.T("license_entry.valid"), msg.licState.Licensee, n), false)
		} else if msg.err != nil {
			s.form.SetMessage(fmt.Sprintf(s.loc.T("license_entry.invalid"), msg.err), true)
		}
		return s, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return s, tea.Quit
		}
	}
	m, cmd := s.form.Update(msg)
	s.form = m.(*simpleForm)
	return s, cmd
}

func (s *licenseEntryScreen) View() string {
	return center(s.Width, s.Height, s.form.View())
}

func readLicenseValue(val string) ([]byte, error) {
	if _, err := os.Stat(val); err == nil {
		return os.ReadFile(val)
	}
	if resolved, ok := resolveLicensePath(val); ok {
		return os.ReadFile(resolved)
	}
	if looksLikeLicensePath(val) {
		return nil, fmt.Errorf("license file not found: %s", val)
	}
	return []byte(val), nil
}

func looksLikeLicensePath(val string) bool {
	if val == "" {
		return false
	}
	if filepath.IsAbs(val) || strings.ContainsRune(val, os.PathSeparator) {
		return true
	}
	return strings.HasSuffix(strings.ToLower(val), ".key")
}

func resolveLicensePath(val string) (string, bool) {
	if !looksLikeLicensePath(val) {
		return "", false
	}
	candidates := []string{val}
	if !filepath.IsAbs(val) {
		if exe, err := os.Executable(); err == nil {
			candidates = append(candidates, filepath.Join(filepath.Dir(exe), val))
		}
		candidates = append(candidates, filepath.Join(defaultLicenseDataDir(), val))
		if home, err := os.UserHomeDir(); err == nil {
			candidates = append(candidates, filepath.Join(home, val))
		}
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, true
		}
	}
	return "", false
}

func defaultLicenseDataDir() string {
	if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
		return filepath.Join(dir, "ztutor")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return filepath.Join(home, ".local", "share", "ztutor")
}
