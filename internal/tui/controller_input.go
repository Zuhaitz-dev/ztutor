package tui

import tea "github.com/charmbracelet/bubbletea"

type gamepadInputMsg struct{ Key string }

var nativeGamepadEnabled bool

func SetNativeGamepadEnabled(enabled bool) {
	nativeGamepadEnabled = enabled
}

var controllerKeyAliases = map[string]string{
	// Common gamepad-to-keyboard mapper profile:
	// Cross/A -> Enter, Circle/B -> Esc, Square/X -> run, Triangle/Y -> hint,
	// D-pad/stick -> arrows.
	"f13": KeySelect,
	"f14": KeyBackAlt,
	"f15": KeyUp,
	"f16": KeyDown,
	"f17": KeyLeft,
	"f18": KeyRight,
	"f19": KeyRun,
	"f20": KeyHintEx,
}

func normalizeInputMsg(msg tea.Msg) tea.Msg {
	if gm, ok := msg.(gamepadInputMsg); ok {
		return keyMsgFromString(gm.Key)
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return msg
	}
	if mapped, ok := controllerKeyAliases[km.String()]; ok {
		return keyMsgFromString(mapped)
	}
	return msg
}

func keyMsgFromString(key string) tea.KeyMsg {
	switch key {
	case KeySelect:
		return tea.KeyMsg{Type: tea.KeyEnter}
	case KeyBackAlt:
		return tea.KeyMsg{Type: tea.KeyEsc}
	case KeyUp:
		return tea.KeyMsg{Type: tea.KeyUp}
	case KeyDown:
		return tea.KeyMsg{Type: tea.KeyDown}
	case KeyLeft:
		return tea.KeyMsg{Type: tea.KeyLeft}
	case KeyRight:
		return tea.KeyMsg{Type: tea.KeyRight}
	case KeySection:
		return tea.KeyMsg{Type: tea.KeyTab}
	case KeySectionPrev:
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	case KeyRun:
		return tea.KeyMsg{Type: tea.KeyCtrlS}
	case KeyHelp:
		return tea.KeyMsg{Type: tea.KeyF1}
	case KeyHintEx:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(KeyHintEx)}
	default:
		r := []rune(key)
		if len(r) == 1 {
			return tea.KeyMsg{Type: tea.KeyRunes, Runes: r}
		}
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
}
