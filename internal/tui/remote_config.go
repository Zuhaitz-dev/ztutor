package tui

import (
	"fmt"
	"strings"

	"ztutor/internal/db"
	"ztutor/internal/i18n"
	"ztutor/internal/remote"

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

type remoteConfigCheckedMsg struct {
	addr  string
	token string
	tls   bool
	err   error
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
	checking   bool
	verified   bool
	status     string
	statusErr  bool
	checked    remoteConfigSavedMsg
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

	case remoteConfigCheckedMsg:
		s.handleCheck(msg)
		return s, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return s, tea.Quit

		case "esc":
			return s, backCmd(NavigateToConnectChoice{})

		case "ctrl+t":
			s.tls = !s.tls
			s.clearCheck()
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
			if s.verified {
				return s, backCmd(s.checked)
			}
			if s.checking {
				return s, nil
			}
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
	oldAddr := s.addrInput.Value()
	oldToken := s.tokenInput.Value()
	s.addrInput, cmd1 = s.addrInput.Update(msg)
	s.tokenInput, cmd2 = s.tokenInput.Update(msg)
	if oldAddr != s.addrInput.Value() || oldToken != s.tokenInput.Value() {
		s.clearCheck()
	}
	return s, tea.Batch(cmd1, cmd2)
}

func (s *remoteConfigScreen) save() tea.Cmd {
	addr := strings.TrimSpace(s.addrInput.Value())
	token := strings.TrimSpace(s.tokenInput.Value())
	s.checked = remoteConfigSavedMsg{addr: addr, token: token, tls: s.tls}

	if addr == "" {
		s.persist(addr, token, s.tls)
		s.verified = true
		s.status = s.loc.T("remote_config.local_ok")
		s.statusErr = false
		return backCmd(s.checked)
	}

	s.checking = true
	s.verified = false
	s.status = s.loc.T("remote_config.wait")
	s.statusErr = false
	return func() tea.Msg {
		err := remote.NewClientWithToken(addr, token, s.tls).Ping()
		return remoteConfigCheckedMsg{addr: addr, token: token, tls: s.tls, err: err}
	}
}

func (s *remoteConfigScreen) persist(addr, token string, tls bool) {
	tlsVal := "1"
	if !tls {
		tlsVal = "0"
	}
	_ = s.db.SetUserSetting(s.username, "exec_addr", addr)
	_ = s.db.SetUserSetting(s.username, "exec_token", token)
	_ = s.db.SetUserSetting(s.username, "exec_tls", tlsVal)
}

func (s *remoteConfigScreen) clearCheck() {
	if s.checking {
		return
	}
	s.verified = false
	s.status = ""
	s.statusErr = false
}

func (s *remoteConfigScreen) handleCheck(msg remoteConfigCheckedMsg) {
	if msg.addr != strings.TrimSpace(s.addrInput.Value()) || msg.token != strings.TrimSpace(s.tokenInput.Value()) || msg.tls != s.tls {
		return
	}
	s.checking = false
	s.checked = remoteConfigSavedMsg{addr: msg.addr, token: msg.token, tls: msg.tls}
	if msg.err != nil {
		s.verified = false
		s.status = fmt.Sprintf(s.loc.T("remote_config.failed"), msg.err)
		s.statusErr = true
		return
	}
	s.persist(msg.addr, msg.token, msg.tls)
	s.verified = true
	s.status = s.loc.T("remote_config.ok")
	s.statusErr = false
}

func (s *remoteConfigScreen) View() string {
	T := s.loc.T

	titleSt := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	labelSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody)).Bold(true)
	dimSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))
	statusSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	if s.statusErr {
		statusSt = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	} else if s.checking {
		statusSt = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorAmber))
	}

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
	b.WriteString("\n\n")
	if s.status != "" {
		b.WriteString(statusSt.Render(s.status))
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")
	}
	help := T("remote_config.save")
	if s.checking {
		help = T("remote_config.wait_help")
	} else if s.verified {
		help = T("remote_config.continue")
	}
	b.WriteString(dimSt.Render(help))

	return center(s.Width, s.Height, b.String())
}
