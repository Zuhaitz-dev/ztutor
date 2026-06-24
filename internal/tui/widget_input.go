package tui

import (
	"strings"
	"ztutor/internal/i18n"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const stdinAreaHeight = 3

// ─── FlagsWidget ─────────────────────────────────────────────────────────────

// FlagsWidget wraps the compiler-flags textinput with its label.
type FlagsWidget struct {
	input textinput.Model
	width int
	loc   *i18n.Locale
}

func newFlagsWidget(width int, loc *i18n.Locale) *FlagsWidget {
	ti := textinput.New()
	ti.Placeholder = loc.T("exercise.placeholder.flags")
	ti.CharLimit = 200
	ti.Prompt = ""
	ti.Width = flagsInputWidth(width)
	return &FlagsWidget{input: ti, width: width, loc: loc}
}

func (w *FlagsWidget) ID() WidgetID             { return WidgetFlags }
func (w *FlagsWidget) Available() bool          { return true }
func (w *FlagsWidget) Focused() bool            { return w.input.Focused() }
func (w *FlagsWidget) Init() tea.Cmd            { return nil }
func (w *FlagsWidget) Focus()                   { w.input.Focus() }
func (w *FlagsWidget) Blur()                    { w.input.Blur() }
func (w *FlagsWidget) Value() string            { return w.input.Value() }
func (w *FlagsWidget) SetLocale(l *i18n.Locale) { w.loc = l }

func (w *FlagsWidget) SetSize(width, _ int) {
	w.width = width
	w.input.Width = flagsInputWidth(width)
}

func (w *FlagsWidget) UpdateInPlace(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.input, cmd = w.input.Update(msg)
	return cmd
}

func (w *FlagsWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	return w, w.UpdateInPlace(msg)
}

func (w *FlagsWidget) View() string {
	T := w.loc.T
	label := flagsLabelStyle.Render(T("exercise.label.flags"))
	if w.input.Focused() {
		label = flagsLabelFocusedStyle.Render(T("exercise.label.flags"))
	}
	return label + w.input.View() + flagsLabelStyle.Render("]")
}

// ─── ArgsWidget ──────────────────────────────────────────────────────────────

// ArgsWidget wraps the runtime-args textinput with its label.
type ArgsWidget struct {
	input textinput.Model
	width int
	loc   *i18n.Locale
}

func newArgsWidget(prefill string, width int, loc *i18n.Locale) *ArgsWidget {
	ti := textinput.New()
	ti.Placeholder = loc.T("exercise.placeholder.args")
	ti.CharLimit = 500
	ti.Prompt = ""
	ti.Width = flagsInputWidth(width)
	ti.SetValue(prefill)
	return &ArgsWidget{input: ti, width: width, loc: loc}
}

func (w *ArgsWidget) ID() WidgetID             { return WidgetArgs }
func (w *ArgsWidget) Available() bool          { return true }
func (w *ArgsWidget) Focused() bool            { return w.input.Focused() }
func (w *ArgsWidget) Init() tea.Cmd            { return nil }
func (w *ArgsWidget) Focus()                   { w.input.Focus() }
func (w *ArgsWidget) Blur()                    { w.input.Blur() }
func (w *ArgsWidget) Value() string            { return w.input.Value() }
func (w *ArgsWidget) SetLocale(l *i18n.Locale) { w.loc = l }

func (w *ArgsWidget) SetSize(width, _ int) {
	w.width = width
	w.input.Width = flagsInputWidth(width)
}

func (w *ArgsWidget) UpdateInPlace(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.input, cmd = w.input.Update(msg)
	return cmd
}

func (w *ArgsWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	return w, w.UpdateInPlace(msg)
}

func (w *ArgsWidget) View() string {
	T := w.loc.T
	label := flagsLabelStyle.Render(T("exercise.label.args"))
	if w.input.Focused() {
		label = flagsLabelFocusedStyle.Render(T("exercise.label.args"))
	}
	return label + w.input.View() + flagsLabelStyle.Render("]")
}

// ─── StdinWidget ─────────────────────────────────────────────────────────────

// StdinWidget wraps the stdin textarea with its label.
type StdinWidget struct {
	area  textarea.Model
	width int
	loc   *i18n.Locale
}

func newStdinWidget(prefill string, width int, loc *i18n.Locale) *StdinWidget {
	ta := textarea.New()
	ta.Placeholder = loc.T("exercise.placeholder.stdin_area")
	ta.ShowLineNumbers = false
	ta.SetHeight(stdinAreaHeight)
	ta.SetValue(prefill)
	ta.SetWidth(flagsInputWidth(width))
	return &StdinWidget{area: ta, width: width, loc: loc}
}

func (w *StdinWidget) ID() WidgetID             { return WidgetStdin }
func (w *StdinWidget) Available() bool          { return true }
func (w *StdinWidget) Focused() bool            { return w.area.Focused() }
func (w *StdinWidget) Init() tea.Cmd            { return nil }
func (w *StdinWidget) Focus()                   { w.area.Focus() }
func (w *StdinWidget) Blur()                    { w.area.Blur() }
func (w *StdinWidget) Value() string            { return w.area.Value() }
func (w *StdinWidget) SetLocale(l *i18n.Locale) { w.loc = l }

func (w *StdinWidget) SetSize(width, _ int) {
	w.width = width
	w.area.SetWidth(flagsInputWidth(width))
}

func (w *StdinWidget) UpdateInPlace(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	w.area, cmd = w.area.Update(msg)
	return cmd
}

func (w *StdinWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	return w, w.UpdateInPlace(msg)
}

func (w *StdinWidget) View() string {
	T := w.loc.T
	if w.area.Focused() {
		return flagsLabelFocusedStyle.Render(T("exercise.label.stdin_focused")) + "\n" + w.area.View()
	}
	// Compact single-line view when not focused: show label + inline content preview.
	preview := w.area.Value()
	if idx := strings.Index(preview, "\n"); idx >= 0 {
		preview = preview[:idx]
	}
	label := flagsLabelStyle.Render(T("exercise.label.stdin_idle"))
	return label + "  " + preview
}
