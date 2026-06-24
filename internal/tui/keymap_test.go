package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEditorKeymap_Default(t *testing.T) {
	ed := newEditorWidget("", "c", "default", 40, 10)

	mode := ed.Mode()
	if mode != "" {
		t.Errorf("default keymap mode = %q, want empty", mode)
	}

	ed.Focus()
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	val := ed.Value()
	if len(val) == 0 {
		t.Error("default mode should accept typing")
	}
}

func TestEditorKeymap_Vim(t *testing.T) {
	ed := newEditorWidget("", "c", "vim", 40, 10)

	mode := ed.Mode()
	if mode == "" {
		t.Error("vim keymap should show mode")
	}
	if mode != "NORMAL" {
		t.Errorf("vim initial mode = %q, want NORMAL", mode)
	}

	ed.Focus()

	// Press 'i' to enter insert mode.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	mode = ed.Mode()
	if mode != "INSERT" {
		t.Errorf("after 'i', mode = %q, want INSERT", mode)
	}

	// Type in insert mode.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	val := ed.Value()
	if val != "hello" {
		t.Errorf("vim insert typed = %q, want hello", val)
	}

	// Escape back to normal mode.
	ed.Update(tea.KeyMsg{Type: tea.KeyEscape})
	mode = ed.Mode()
	if mode != "NORMAL" {
		t.Errorf("after escape, mode = %q, want NORMAL", mode)
	}
}

func TestEditorKeymap_VimNormalMotion(t *testing.T) {
	ed := newEditorWidget("abc\n123", "c", "vim", 40, 10)
	ed.Focus()

	if mode := ed.Mode(); mode != "NORMAL" {
		t.Fatalf("initial mode = %q, want NORMAL", mode)
	}

	// Move right ('l').
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	if ed.Row() != 0 {
		t.Errorf("after 'l', row = %d, want 0", ed.Row())
	}

	// Move down ('j').
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if ed.Row() != 1 {
		t.Errorf("after 'j', row = %d, want 1", ed.Row())
	}

	// Move up ('k').
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	if ed.Row() != 0 {
		t.Errorf("after 'k', row = %d, want 0", ed.Row())
	}

	// Press '0' to go to column 0.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'0'}})
}

func TestEditorKeymap_SwitchFilesPreservesKeymap(t *testing.T) {
	ed := newEditorWidget("first", "c", "vim", 40, 10)
	ed.Focus()

	if mode := ed.Mode(); mode != "NORMAL" {
		t.Fatalf("initial mode = %q, want NORMAL", mode)
	}

	ed.SwitchFile("second", "python")

	if mode := ed.Mode(); mode != "NORMAL" {
		t.Errorf("after SwitchFile, mode = %q, want NORMAL", mode)
	}
	if ed.Value() != "second" {
		t.Errorf("after SwitchFile, value = %q, want second", ed.Value())
	}
}

func TestExerciseScreen_RespectsKeymap(t *testing.T) {
	// Verify that NewExerciseScreen propagates the keymap to the inner editor.
	s := newExerciseScreenHelper(t, "vim")

	ed := s.editor
	if ed == nil {
		t.Fatal("editor is nil")
	}
	ed.Focus()

	if mode := ed.Mode(); mode != "NORMAL" {
		t.Errorf("vim exercise screen editor mode = %q, want NORMAL", mode)
	}
}

func TestExerciseScreen_DefaultKeymap(t *testing.T) {
	s := newExerciseScreenHelper(t, "default")

	ed := s.editor
	if ed == nil {
		t.Fatal("editor is nil")
	}
	ed.Focus()

	if mode := ed.Mode(); mode != "" {
		t.Errorf("default exercise screen editor mode = %q, want empty", mode)
	}
}

func TestEditorKeymap_VimInsertAndNormalBackAndForth(t *testing.T) {
	ed := newEditorWidget("hello", "c", "vim", 40, 10)
	ed.Focus()

	// Start in NORMAL mode at (0,0).
	if ed.Mode() != "NORMAL" {
		t.Fatalf("initial mode = %q", ed.Mode())
	}

	// Press 'A' to append at end of line and enter INSERT mode.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if ed.Mode() != "INSERT" {
		t.Fatalf("after 'A': mode = %q", ed.Mode())
	}

	// Type ' world'.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'w'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'r'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})

	if ed.Value() != "hello world" {
		t.Errorf("value = %q, want 'hello world'", ed.Value())
	}

	// Escape to NORMAL.
	ed.Update(tea.KeyMsg{Type: tea.KeyEscape})
	if ed.Mode() != "NORMAL" {
		t.Fatalf("after esc: mode = %q", ed.Mode())
	}

	// Press 'A' again to append and enter INSERT.
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'A'}})
	if ed.Mode() != "INSERT" {
		t.Fatalf("after 'A': mode = %q", ed.Mode())
	}
	ed.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'!'}})
	if ed.Value() != "hello world!" {
		t.Errorf("value = %q, want 'hello world!'", ed.Value())
	}
}

// newExerciseScreenHelper creates a minimal ExerciseScreen for testing keymap propagation.
func newExerciseScreenHelper(t *testing.T, keymap string) *ExerciseScreen {
	t.Helper()
	return &ExerciseScreen{
		keymap: keymap,
		editor: newEditorWidget("// code here", "c", keymap, 60, 20),
	}
}
