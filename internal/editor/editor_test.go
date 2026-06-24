package editor

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew(t *testing.T) {
	ed := New("hello\nworld", 80, 20, "c")
	if ed.Value() != "hello\nworld" {
		t.Errorf("Value = %q, want %q", ed.Value(), "hello\nworld")
	}
	if ed.Row() != 0 {
		t.Errorf("Row = %d, want 0", ed.Row())
	}
}

func TestEditorInsert(t *testing.T) {
	ed := New("abc", 80, 20, "c")
	ed.Focus()
	ed.col = 3

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	if ed.Value() != "abcd" {
		t.Errorf("Value = %q, want %q", ed.Value(), "abcd")
	}
}

func TestEditorBackspace(t *testing.T) {
	ed := New("abc", 80, 20, "c")
	ed.Focus()
	ed.row, ed.col = 0, 3

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if ed.Value() != "ab" {
		t.Errorf("Value = %q, want %q", ed.Value(), "ab")
	}
	if ed.col != 2 {
		t.Errorf("col = %d, want 2", ed.col)
	}
}

func TestEditorEnter(t *testing.T) {
	ed := New("abc", 80, 20, "c")
	ed.Focus()
	ed.row, ed.col = 0, 1

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if ed.Value() != "a\nbc" {
		t.Errorf("Value = %q, want %q", ed.Value(), "a\nbc")
	}
	if ed.row != 1 {
		t.Errorf("row = %d, want 1", ed.row)
	}
}

func TestEditorVimBasic(t *testing.T) {
	ed := New("hello world", 80, 20, "c")
	ed.SetVimMode(true)
	ed.Focus()

	if ed.mode != modeNormal {
		t.Errorf("mode = %d, want modeNormal", ed.mode)
	}
	if ed.Mode() != "NORMAL" {
		t.Errorf("Mode() = %q, want %q", ed.Mode(), "NORMAL")
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	if ed.mode != modeInsert {
		t.Errorf("mode after i = %d, want modeInsert", ed.mode)
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if ed.mode != modeNormal {
		t.Errorf("mode after esc = %d, want modeNormal", ed.mode)
	}
}

func TestEditorVimMotion(t *testing.T) {
	ed := New("abc\ndef\nghi", 80, 20, "c")
	ed.SetVimMode(true)
	ed.Focus()

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if ed.row != 1 {
		t.Errorf("row after j = %d, want 1", ed.row)
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if ed.row != 0 {
		t.Errorf("row after k = %d, want 0", ed.row)
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if ed.col != 1 {
		t.Errorf("col after l = %d, want 1", ed.col)
	}

	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if ed.col != 0 {
		t.Errorf("col after h = %d, want 0", ed.col)
	}
}

func TestEditorVimDelete(t *testing.T) {
	ed := New("line1\nline2\nline3", 80, 20, "c")
	ed.SetVimMode(true)
	ed.Focus()

	ed.row, ed.col = 0, 0
	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if ed.Value() != "line2\nline3" {
		t.Errorf("Value after dd = %q", ed.Value())
	}
	if len(ed.register) != 1 || string(ed.register[0]) != "line1" {
		t.Errorf("register after dd = %v", ed.register)
	}
}

func TestEditorVimUndoRedo(t *testing.T) {
	ed := New("hello", 80, 20, "c")
	ed.SetVimMode(true)
	ed.Focus()

	ed.pushUndo()
	ed.lines[0] = []rune("world")
	ed.col = 0

	ed.undo()
	if ed.Value() != "hello" {
		t.Errorf("Value after undo = %q, want %q", ed.Value(), "hello")
	}

	ed.redo()
	if ed.Value() != "world" {
		t.Errorf("Value after redo = %q, want %q", ed.Value(), "world")
	}
}

func TestEditorVimPaste(t *testing.T) {
	ed := New("line1\n// gap\nline3", 80, 20, "c")
	ed.SetVimMode(true)
	ed.Focus()

	ed.row, ed.col = 0, 0
	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	ed.row = 1
	ed, _ = ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})

	lines := strings.Split(ed.Value(), "\n")
	if len(lines) != 4 || lines[0] != "line1" || lines[1] != "// gap" || lines[2] != "line1" || lines[3] != "line3" {
		t.Errorf("Value after p = %q", ed.Value())
	}
}

func TestEditorUndoMaxSteps(t *testing.T) {
	ed := New("initial", 80, 20, "c")
	ed.Focus()

	for i := 0; i < 250; i++ {
		ed.pushUndo()
	}

	if len(ed.undoStack) > maxUndoSteps {
		t.Errorf("undoStack length = %d, max = %d", len(ed.undoStack), maxUndoSteps)
	}
}

func TestEditorDiagnostics(t *testing.T) {
	ed := New("line1\nline2\nline3", 80, 20, "c")
	ed.Focus()

	ed.Diagnostics = map[int]string{
		2: "error",
		3: "warning",
	}

	if ed.Diagnostics[2] != "error" {
		t.Errorf("Diagnostics[2] = %q, want %q", ed.Diagnostics[2], "error")
	}
	if ed.Diagnostics[3] != "warning" {
		t.Errorf("Diagnostics[3] = %q, want %q", ed.Diagnostics[3], "warning")
	}
}
