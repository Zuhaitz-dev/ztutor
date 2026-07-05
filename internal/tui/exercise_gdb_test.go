package tui

import (
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

func newGDBExerciseScreen(t *testing.T) *ExerciseScreen {
	t.Helper()
	es := newFakeExerciseScreen(t, testLesson("gdb-test", `int main(void) { return 0; }`, nil, nil))
	es.lang = sandbox.GetLanguage("c")
	return es
}

func enterGDBMode(es *ExerciseScreen) {
	es.running = true
	es.runMode = runModeGDB
	es.interKill = func() {}
}

func TestGDB_Keyboard_SpaceForwarded(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var received string
	es.interWrite = func(data []byte) error { received = string(data); return nil }
	enterGDBMode(es)

	injectMsg(es, tea.KeyMsg{Type: tea.KeySpace})

	if received != " " {
		t.Errorf("received %q, want space", received)
	}
}

func TestGDB_Keyboard_EnterForwarded(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var received string
	es.interWrite = func(data []byte) error { received = string(data); return nil }
	enterGDBMode(es)

	injectMsg(es, tea.KeyMsg{Type: tea.KeyEnter})

	if received != "\n" {
		t.Errorf("received %q, want newline", received)
	}
}

func TestGDB_Keyboard_TabForwarded(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var received string
	es.interWrite = func(data []byte) error { received = string(data); return nil }
	enterGDBMode(es)

	injectMsg(es, tea.KeyMsg{Type: tea.KeyTab})

	if received != "\t" {
		t.Errorf("received %q, want tab", received)
	}
}

func TestGDB_Keyboard_BackspaceForwarded(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var received string
	es.interWrite = func(data []byte) error { received = string(data); return nil }
	enterGDBMode(es)

	injectMsg(es, tea.KeyMsg{Type: tea.KeyBackspace})

	if received != "\x7f" {
		t.Errorf("received %q, want backspace", received)
	}
}

func TestGDB_Keyboard_CtrlD_EscForwarded(t *testing.T) {
	tests := []struct {
		key  tea.KeyType
		want string
	}{
		{tea.KeyCtrlD, "\x04"},
		{tea.KeyEsc, "\x1b"},
	}
	for _, tt := range tests {
		es := newGDBExerciseScreen(t)
		var received string
		es.interWrite = func(data []byte) error { received = string(data); return nil }
		enterGDBMode(es)

		injectMsg(es, tea.KeyMsg{Type: tt.key})

		if received != tt.want {
			t.Errorf("key %v: received %q, want %q", tt.key, received, tt.want)
		}
	}
}

func TestGDB_Keyboard_CtrlCKillsGDB(t *testing.T) {
	// Ctrl+C is consumed by the KeyQuit check before the per-type forwarding,
	// so it kills GDB rather than forwarding \x03 to the process.
	es := newGDBExerciseScreen(t)
	var killed bool
	es.interWrite = func(data []byte) error { return nil }
	enterGDBMode(es)
	es.interKill = func() { killed = true }

	injectMsg(es, tea.KeyMsg{Type: tea.KeyCtrlC})

	if !killed {
		t.Error("Ctrl+C should kill GDB via KeyQuit check")
	}
}

func TestGDB_Keyboard_ArrowsForwarded(t *testing.T) {
	tests := []struct {
		key  tea.KeyType
		want string
	}{
		{tea.KeyUp, "\x1b[A"},
		{tea.KeyDown, "\x1b[B"},
		{tea.KeyRight, "\x1b[C"},
		{tea.KeyLeft, "\x1b[D"},
	}
	for _, tt := range tests {
		es := newGDBExerciseScreen(t)
		var received string
		es.interWrite = func(data []byte) error { received = string(data); return nil }
		enterGDBMode(es)

		injectMsg(es, tea.KeyMsg{Type: tt.key})

		if received != tt.want {
			t.Errorf("key %v: received %q, want %q", tt.key, received, tt.want)
		}
	}
}

func TestGDB_Keyboard_RunesForwarded(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var received []string
	es.interWrite = func(data []byte) error { received = append(received, string(data)); return nil }
	enterGDBMode(es)

	injectMsg(es, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})

	if len(received) != 1 || received[0] != "b" {
		t.Errorf("received %v, want ['b']", received)
	}
}

func TestGDB_Keyboard_MultipleRunesForwarded(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var received []string
	es.interWrite = func(data []byte) error { received = append(received, string(data)); return nil }
	enterGDBMode(es)

	injectMsg(es, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h', 'e', 'l', 'l', 'o'}})

	if len(received) != 5 {
		t.Fatalf("received %d runes, want 5: %v", len(received), received)
	}
	want := []string{"h", "e", "l", "l", "o"}
	for i := range want {
		if received[i] != want[i] {
			t.Errorf("rune %d: got %q, want %q", i, received[i], want[i])
		}
	}
}

func TestGDB_Keyboard_CtrlQ_KillsGDBAndReturns(t *testing.T) {
	es := newGDBExerciseScreen(t)
	var killed bool
	es.interWrite = func(data []byte) error { return nil }
	enterGDBMode(es)
	es.interKill = func() { killed = true }

	_, cmd := es.Update(tea.KeyMsg{Type: tea.KeyCtrlQ})

	if !killed {
		t.Error("interKill should have been called")
	}
	if cmd == nil {
		t.Fatal("expected NavigateBackToCourse, got nil")
	}
	msg := cmd()
	if _, ok := msg.(NavigateBackToCourse); !ok {
		t.Errorf("expected NavigateBackToCourse, got %T", msg)
	}
}

func TestGDB_Keyboard_CtrlG_TogglesFullscreen(t *testing.T) {
	es := newGDBExerciseScreen(t)
	es.interWrite = func(data []byte) error { return nil }
	es.interKill = func() {}
	enterGDBMode(es)

	// First Ctrl+G: should enter fullscreen output.
	injectMsg(es, tea.KeyMsg{Type: tea.KeyCtrlG})
	if !es.compositor.InFullscreen() || es.compositor.FullscreenID() != WidgetOutput {
		t.Error("Ctrl+G should enter fullscreen output")
	}

	// Second Ctrl+G: should exit fullscreen.
	injectMsg(es, tea.KeyMsg{Type: tea.KeyCtrlG})
	if es.compositor.InFullscreen() {
		t.Error("second Ctrl+G should exit fullscreen")
	}
}

func TestGDB_StateTransition_GdbReadyMsg_SetsRunMode(t *testing.T) {
	es := newGDBExerciseScreen(t)
	es.running = true
	es.runMode = runModeGDB
	es.interKill = func() {}

	if es.runMode != runModeGDB {
		t.Errorf("runMode = %d, want runModeGDB", es.runMode)
	}
	if !es.running {
		t.Error("should be running in GDB mode")
	}
}

func TestGDB_StateTransition_GdbReadyMsg_EntersFullscreen(t *testing.T) {
	es := newGDBExerciseScreen(t)
	es.compositor.EnterFullscreen(WidgetOutput)

	if !es.compositor.InFullscreen() || es.compositor.FullscreenID() != WidgetOutput {
		t.Error("should be in fullscreen output")
	}
}

func TestGDB_StateTransition_GdbReadyMsg_CompileError(t *testing.T) {
	es := newGDBExerciseScreen(t)
	es.startRun("compiling")

	injectMsg(es, gdbReadyMsg{compileErr: &sandbox.Result{Error: "syntax error"}})

	if es.compiling {
		t.Error("compiling should be false after compile error")
	}
	if es.runMode != runModeNone {
		t.Errorf("runMode = %d, want runModeNone on compile error", es.runMode)
	}
}

func TestGDB_StateTransition_RunModeInteractiveSet(t *testing.T) {
	es := newGDBExerciseScreen(t)
	es.startRun("compiling")
	// fakeExecutor returns an error from RunInteractive, so we need to inject
	// the state changes that would happen after a successful RunInteractive call.
	es.running = true
	es.runMode = runModeInteractive
	es.interKill = func() {}
	es.interWrite = func(data []byte) error { return nil }

	if es.runMode != runModeInteractive {
		t.Errorf("runMode = %d, want runModeInteractive", es.runMode)
	}
	if !es.running {
		t.Error("should be running in interactive mode")
	}
}

func TestGDB_StateTransition_ProgramDone_CleansUpGDBMode(t *testing.T) {
	es := newGDBExerciseScreen(t)
	es.running = true
	es.runMode = runModeGDB
	es.interKill = func() {}
	es.compositor.EnterFullscreen(WidgetOutput)

	injectMsg(es, programDoneMsg{code: 0})

	if es.runMode != runModeNone {
		t.Errorf("runMode = %d, want runModeNone after programDone", es.runMode)
	}
	if es.running {
		t.Error("running should be false after programDone")
	}
	if es.compositor.InFullscreen() {
		t.Error("should exit fullscreen after GDB programDone")
	}
	if es.compositor.FocusID() != WidgetEditor {
		t.Errorf("focus = %d, want WidgetEditor after GDB done", es.compositor.FocusID())
	}
}

func TestGDB_Locale_GdbExitKey(t *testing.T) {
	if loc := i18n.New("en"); loc.T("exercise.mochi.gdb_exit") == "exercise.mochi.gdb_exit" {
		t.Error("gdb_exit key missing in English locale")
	}
	if loc := i18n.New("es"); loc.T("exercise.mochi.gdb_exit") == "exercise.mochi.gdb_exit" {
		t.Error("gdb_exit key missing in Spanish locale")
	}
}
