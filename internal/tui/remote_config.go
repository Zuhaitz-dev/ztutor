package tui

import (
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/i18n"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NavigateToRemoteConfig navigates to the execution server configuration form.
type NavigateToRemoteConfig struct{}

// remoteConfigSavedMsg is emitted when the user saves the remote execution config.
type remoteConfigSavedMsg struct {
	addr  string
	token string
	tls   bool
}

const (
	rcFocusAddr  = 0
	rcFocusToken = 1
)

type remoteConfigScreen struct {
	loc *i18n.Locale
	sized
	db       *db.DB
	username string

	addrInput  textinput.Model
	tokenInput textinput.Model
	focused    int
	tls        bool
}

func NewRemoteConfigScreen(loc *i18n.Locale, w, h int, database *db.DB, username string) *remoteConfigScreen {
	addrIn := textinput.New()
	addrIn.Placeholder = loc.T("remote_config.addr_placeholder")
	addrIn.Width = 42
	addrIn.CharLimit = 256
	addrIn.Focus()

	tokenIn := textinput.New()
	tokenIn.Placeholder = loc.T("remote_config.token_placeholder")
	tokenIn.Width = 42
	tokenIn.CharLimit = 512
	tokenIn.EchoMode = textinput.EchoPassword
	tokenIn.EchoCharacter = '•'

	// Pre-fill from DB
	if addr, _ := database.GetUserSetting(username, "exec_addr"); addr != "" {
		addrIn.SetValue(addr)
	}
	if token, _ := database.GetUserSetting(username, "exec_token"); token != "" {
		tokenIn.SetValue(token)
	}

	tlsStr, _ := database.GetUserSetting(username, "exec_tls")
	useTLS := tlsStr != "0" // empty = default on

	return &remoteConfigScreen{
		loc:        loc,
		sized:      sized{Width: w, Height: h},
		db:         database,
		username:   username,
		addrInput:  addrIn,
		tokenInput: tokenIn,
		tls:        useTLS,
	}
}

func (s *remoteConfigScreen) Init() tea.Cmd { return textinput.Blink }

func (s *remoteConfigScreen) SetLocale(loc *i18n.Locale) { s.loc = loc }

func (s *remoteConfigScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		s.HandleResize(msg)

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return s, tea.Quit

		case "esc":
			return s, backCmd(NavigateToConnectChoice{})

		case "ctrl+t":
			s.tls = !s.tls
			return s, nil

		case "tab":
			if s.focused == rcFocusAddr {
				s.focused = rcFocusToken
				s.addrInput.Blur()
				s.tokenInput.Focus()
			} else {
				s.focused = rcFocusAddr
				s.tokenInput.Blur()
				s.addrInput.Focus()
			}
			return s, textinput.Blink

		case "shift+tab":
			if s.focused == rcFocusToken {
				s.focused = rcFocusAddr
				s.tokenInput.Blur()
				s.addrInput.Focus()
			} else {
				s.focused = rcFocusToken
				s.addrInput.Blur()
				s.tokenInput.Focus()
			}
			return s, textinput.Blink

		case "enter":
			if s.focused == rcFocusAddr {
				// advance to token field
				s.focused = rcFocusToken
				s.addrInput.Blur()
				s.tokenInput.Focus()
				return s, textinput.Blink
			}
			return s, s.save()
		}
	}

	var cmd1, cmd2 tea.Cmd
	s.addrInput, cmd1 = s.addrInput.Update(msg)
	s.tokenInput, cmd2 = s.tokenInput.Update(msg)
	return s, tea.Batch(cmd1, cmd2)
}

func (s *remoteConfigScreen) save() tea.Cmd {
	addr := strings.TrimSpace(s.addrInput.Value())
	token := strings.TrimSpace(s.tokenInput.Value())
	tlsVal := "1"
	if !s.tls {
		tlsVal = "0"
	}
	_ = s.db.SetUserSetting(s.username, "exec_addr", addr)
	_ = s.db.SetUserSetting(s.username, "exec_token", token)
	_ = s.db.SetUserSetting(s.username, "exec_tls", tlsVal)
	return func() tea.Msg {
		return remoteConfigSavedMsg{addr: addr, token: token, tls: s.tls}
	}
}

func (s *remoteConfigScreen) View() string {
	T := s.loc.T

	titleSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	labelSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true)
	dimSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))

	tlsLabel := T("remote_config.tls_on")
	if !s.tls {
		tlsLabel = T("remote_config.tls_off")
	}

	var b strings.Builder
	b.WriteString(titleSt.Render(T("remote_config.title")))
	b.WriteString("\n\n")
	b.WriteString(dimSt.Render(T("remote_config.subtitle")))
	b.WriteString("\n\n")
	b.WriteString(labelSt.Render(T("remote_config.addr_label")))
	b.WriteString("\n")
	b.WriteString(s.addrInput.View())
	b.WriteString("\n\n")
	b.WriteString(labelSt.Render(T("remote_config.token_label")))
	b.WriteString("\n")
	b.WriteString(s.tokenInput.View())
	b.WriteString("\n\n")
	b.WriteString(dimSt.Render(tlsLabel + "  " + T("remote_config.tls_hint")))
	b.WriteString("\n\n\n")
	b.WriteString(dimSt.Render(T("remote_config.save")))

	return center(s.Width, s.Height, b.String())
}
