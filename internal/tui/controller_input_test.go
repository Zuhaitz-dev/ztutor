package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNormalizeInputMsg_ControllerAliases(t *testing.T) {
	cases := []struct {
		name string
		in   tea.KeyMsg
		want string
	}{
		{name: "select", in: tea.KeyMsg{Type: tea.KeyF13}, want: KeySelect},
		{name: "back", in: tea.KeyMsg{Type: tea.KeyF14}, want: KeyBackAlt},
		{name: "up", in: tea.KeyMsg{Type: tea.KeyF15}, want: KeyUp},
		{name: "run", in: tea.KeyMsg{Type: tea.KeyF19}, want: KeyRun},
		{name: "hint", in: tea.KeyMsg{Type: tea.KeyF20}, want: KeyHintEx},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := normalizeInputMsg(tc.in).(tea.KeyMsg)
			if !ok {
				t.Fatal("normalized message is not a key message")
			}
			if got.String() != tc.want {
				t.Fatalf("normalized key = %q, want %q", got.String(), tc.want)
			}
		})
	}
}

func TestNormalizeInputMsg_LeavesKeyboardInputAlone(t *testing.T) {
	in := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}}
	got := normalizeInputMsg(in).(tea.KeyMsg)
	if got.String() != KeyDownVim {
		t.Fatalf("keyboard key changed to %q", got.String())
	}
}
