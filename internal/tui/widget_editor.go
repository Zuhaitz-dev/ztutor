package tui

import (
	editormod "ztutor/internal/editor"

	tea "github.com/charmbracelet/bubbletea"
)

// EditorWidget wraps editormod.CodeEditor and supports per-file content and syntax
// switching without replacing the outer ExerciseScreen struct fields.
type EditorWidget struct {
	editor  *editormod.CodeEditor
	keymap  string
	focused bool
	width   int
	height  int
}

func newEditorWidget(content, hlLang, keymap string, width, height int) *EditorWidget {
	if hlLang == "" {
		hlLang = "c"
	}
	ed := editormod.New(content, width, height, hlLang)
	ed.SetVimMode(keymap == "vim")
	return &EditorWidget{
		editor: ed,
		keymap: keymap,
		width:  width,
		height: height,
	}
}

func (w *EditorWidget) ID() WidgetID    { return WidgetEditor }
func (w *EditorWidget) Available() bool { return true }
func (w *EditorWidget) Focused() bool   { return w.focused }
func (w *EditorWidget) Init() tea.Cmd   { return nil }

func (w *EditorWidget) Focus() {
	w.focused = true
	w.editor.Focus()
}

func (w *EditorWidget) Blur() {
	w.focused = false
	w.editor.Blur()
}

func (w *EditorWidget) SetSize(width, height int) {
	w.width, w.height = width, height
	w.editor.SetSize(width, height)
}

// SwitchFile replaces the editor with new content and syntax highlighting.
// The cursor resets to the top of the new file.
func (w *EditorWidget) SwitchFile(content, hlLang string) {
	if hlLang == "" {
		hlLang = "text"
	}
	ed := editormod.New(content, w.width, w.height, hlLang)
	ed.SetVimMode(w.keymap == "vim")
	if w.focused {
		ed.Focus()
	}
	w.editor = ed
}

// UpdateInPlace handles a tea.Msg, updates the inner editormod.CodeEditor in place, and
// returns the command. Use this instead of Widget.Update to avoid a type assertion.
func (w *EditorWidget) UpdateInPlace(msg tea.Msg) tea.Cmd {
	newEd, cmd := w.editor.Update(msg)
	w.editor = newEd
	return cmd
}

// Update satisfies the Widget interface; prefer UpdateInPlace for direct use.
func (w *EditorWidget) Update(msg tea.Msg) (Widget, tea.Cmd) {
	cmd := w.UpdateInPlace(msg)
	return w, cmd
}

func (w *EditorWidget) View() string {
	return w.editor.View()
}

// Value returns the current editor content.
func (w *EditorWidget) Value() string { return w.editor.Value() }

// Row returns the 0-indexed cursor row.
func (w *EditorWidget) Row() int { return w.editor.Row() }

// Mode returns the current vim mode label (empty string when not in vim mode).
func (w *EditorWidget) Mode() string { return w.editor.Mode() }

// SetDiagnostics updates the gutter diagnostic map (line → kind).
func (w *EditorWidget) SetDiagnostics(m map[int]string) {
	w.editor.Diagnostics = m
}
