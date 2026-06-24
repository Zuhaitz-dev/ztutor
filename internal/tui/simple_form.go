package tui

import (
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// simpleForm wraps a single textinput with enter-to-submit and esc-to-back.
type simpleForm struct {
	input    textinput.Model
	title    string
	prompt   string
	help     string
	msg      string
	isError  bool
	onSubmit func(string) tea.Cmd
	backMsg  tea.Msg
}

func newSimpleForm(placeholder, title, prompt, help string, onSubmit func(string) tea.Cmd, backMsg tea.Msg) *simpleForm {
	ti := textinput.New()
	ti.Placeholder = placeholder
	ti.Width = 50
	ti.Focus()
	return &simpleForm{
		input:    ti,
		title:    title,
		prompt:   prompt,
		help:     help,
		onSubmit: onSubmit,
		backMsg:  backMsg,
	}
}

func (f *simpleForm) Init() tea.Cmd { return textinput.Blink }

func (f *simpleForm) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			val := strings.TrimSpace(f.input.Value())
			if val == "" {
				return f, nil
			}
			return f, f.onSubmit(val)
		case "esc":
			return f, backCmd(f.backMsg)
		default:
			var cmd tea.Cmd
			f.input, cmd = f.input.Update(msg)
			return f, cmd
		}
	}
	return f, nil
}

func (f *simpleForm) SetMessage(msg string, isError bool) {
	f.msg = msg
	f.isError = isError
}

func (f *simpleForm) View() string {
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAccent))
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorBody))
	msgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	if f.isError {
		msgStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	}
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorDim))

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(titleStyle.Render("  " + f.title))
	b.WriteString("\n\n")
	b.WriteString(promptStyle.Render("  " + f.prompt))
	b.WriteString("\n\n")
	b.WriteString("  " + f.input.View())
	b.WriteString("\n\n")
	if f.msg != "" {
		b.WriteString("  " + msgStyle.Render(f.msg))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(descStyle.Render("  " + f.help))
	return b.String()
}
