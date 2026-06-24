package tui

import (
	"fmt"
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

// fakeExecutor satisfies sandbox.Executor but returns errors for everything.
// Tests inject messages directly into ExerciseScreen.Update without needing
// real compilation — fakeExecutor just prevents panics in the rare handler
// paths that still call into the executor.
type fakeExecutor struct{}

func (fakeExecutor) Run(_ sandbox.Language, _ map[string]string, _, _ string, _, _ []string) (*sandbox.Result, error) {
	return nil, fmt.Errorf("fake")
}
func (fakeExecutor) RunWithASAN(_ sandbox.Language, _ map[string]string, _, _ string, _, _ []string) (*sandbox.Result, error) {
	return nil, fmt.Errorf("fake")
}
func (fakeExecutor) CompileDebug(_ sandbox.Language, _ map[string]string, _ string, _ []string) (*sandbox.DebugBuild, *sandbox.Result) {
	return nil, &sandbox.Result{Error: "fake"}
}
func (fakeExecutor) GenerateAssembly(_ sandbox.Language, _ map[string]string, _ string, _ []string) (string, error) {
	return "", fmt.Errorf("fake")
}
func (fakeExecutor) SyntaxCheck(_ sandbox.Language, _ map[string]string, _ string, _ []string) ([]sandbox.Diagnostic, error) {
	return nil, fmt.Errorf("fake")
}
func (fakeExecutor) RunAllTests(_ sandbox.Language, _ map[string]string, _ string, _ []string, _ []sandbox.TestInput) (*sandbox.Result, []sandbox.TestResult, error) {
	return nil, nil, fmt.Errorf("fake")
}
func (fakeExecutor) RunInteractive(_ string, _ []string) (func([]byte) error, <-chan sandbox.InteractiveEvent, func(), error) {
	return nil, nil, func() {}, fmt.Errorf("fake")
}

func newFakeExerciseScreen(t *testing.T, l lesson.Lesson) *ExerciseScreen {
	t.Helper()
	lang := sandbox.GetLanguage("c")
	loc := i18n.New("en")
	return NewExerciseScreen(l, lang, fakeExecutor{}, 80, 40, "vim", 0, 0, loc, true, true)
}

func newMultiFileLesson() lesson.Lesson {
	return testLesson("mf", "", nil, []lesson.ExerciseFile{
		{Name: "main.c", Language: "c", Editable: true, Content: "// main"},
		{Name: "utils.h", Language: "c", Editable: false, Content: "// header"},
	})
}

func injectMsg(es *ExerciseScreen, msg tea.Msg) {
	_, _ = es.Update(msg)
}

// ── interactiveReadyMsg ───────────────────────────────────────────────────────

// Regression: interactiveReadyMsg never called timer.Stop(), so the timer kept
// running for the whole interactive session instead of recording compile time.

func TestMsg_InteractiveReady_StopsTimer_OnCompileError(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("compiling")
	if !es.timer.running {
		t.Fatal("setup: timer should be running after startRun")
	}
	injectMsg(es, interactiveReadyMsg{compileErr: &sandbox.Result{Error: "syntax error"}})
	if es.timer.running {
		t.Error("timer should stop after interactiveReadyMsg compile error")
	}
}

func TestMsg_InteractiveReady_StopsTimer_OnSuccess(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("compiling")
	// fakeExecutor.RunInteractive returns an error, so the handler exits after
	// stopping the timer. The timer stop happens before RunInteractive is called.
	injectMsg(es, interactiveReadyMsg{build: &sandbox.DebugBuild{}})
	if es.timer.running {
		t.Error("timer should stop after interactiveReadyMsg success path")
	}
}

func TestMsg_InteractiveReady_CompileError_ClearsCompiling(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("compiling")
	if !es.compiling {
		t.Fatal("setup: compiling should be true after startRun")
	}
	injectMsg(es, interactiveReadyMsg{compileErr: &sandbox.Result{Error: "err"}})
	if es.compiling {
		t.Error("compiling should be cleared after interactiveReadyMsg")
	}
	if es.passed {
		t.Error("passed should be false after compile error")
	}
}

// ── programDoneMsg ────────────────────────────────────────────────────────────

func TestMsg_ProgramDone_ClearsRunningAndFocusesEditor(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.running = true
	injectMsg(es, programDoneMsg{code: 0})
	if es.running {
		t.Error("running should be cleared after programDoneMsg")
	}
	if es.compositor.FocusID() != WidgetEditor {
		t.Errorf("focus = %d, want WidgetEditor after program done", es.compositor.FocusID())
	}
}

func TestMsg_ProgramDone_NonZeroSetsFailed(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.running = true
	es.passed = true
	injectMsg(es, programDoneMsg{code: 1})
	if es.passed {
		t.Error("non-zero exit code should set passed=false")
	}
}

func TestMsg_ProgramDone_Segfault_SetsFailed(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.running = true
	es.passed = true
	injectMsg(es, programDoneMsg{code: 139})
	if es.passed {
		t.Error("segfault (exit 139) should set passed=false")
	}
}

// ── fileSelectedMsg ───────────────────────────────────────────────────────────

func TestMsg_FileSelected_SwitchesActiveIndex(t *testing.T) {
	es := newFakeExerciseScreen(t, newMultiFileLesson())
	if es.activeIdx != 0 {
		t.Fatal("setup: initial activeIdx should be 0")
	}
	injectMsg(es, fileSelectedMsg{idx: 1})
	if es.activeIdx != 1 {
		t.Errorf("activeIdx = %d, want 1", es.activeIdx)
	}
}

func TestMsg_FileSelected_NormalMode_FocusesEditor(t *testing.T) {
	es := newFakeExerciseScreen(t, newMultiFileLesson())
	es.compositor.FocusFileList()
	injectMsg(es, fileSelectedMsg{idx: 1})
	if es.compositor.FocusID() != WidgetEditor {
		t.Errorf("focus = %d, want WidgetEditor after file switch in normal mode", es.compositor.FocusID())
	}
	if es.compositor.InFullscreen() {
		t.Error("should not enter fullscreen after file switch in normal mode")
	}
}

// Regression: fileSelectedMsg called FocusEditor() unconditionally, which
// called resetLayout() and exited fullscreen when switching files in multifile.
func TestMsg_FileSelected_PreservesFullscreenEditor(t *testing.T) {
	es := newFakeExerciseScreen(t, newMultiFileLesson())
	es.compositor.EnterFullscreen(WidgetEditor)
	if !es.compositor.InFullscreen() {
		t.Fatal("setup: should be in fullscreen editor")
	}
	injectMsg(es, fileSelectedMsg{idx: 1})
	if !es.compositor.InFullscreen() {
		t.Error("file switch should preserve fullscreen editor layout")
	}
	if es.compositor.FullscreenID() != WidgetEditor {
		t.Errorf("fullscreen widget = %d, want WidgetEditor after file switch", es.compositor.FullscreenID())
	}
	if es.compositor.FocusID() != WidgetEditor {
		t.Errorf("focus = %d, want WidgetEditor after file switch", es.compositor.FocusID())
	}
}

func TestMsg_FileSelected_ExitsAsmSplit(t *testing.T) {
	es := newFakeExerciseScreen(t, newMultiFileLesson())
	es.assembly.Open([]string{"push rbp"}, "")
	es.compositor.FocusAsm()
	if !es.compositor.InAsmMode() {
		t.Fatal("setup: should be in asm split")
	}
	injectMsg(es, fileSelectedMsg{idx: 1})
	if es.compositor.InAsmMode() {
		t.Error("file switch should exit asm split (assembly is stale for the new file)")
	}
	if es.compositor.FocusID() != WidgetEditor {
		t.Error("focus should be on editor after file switch from asm split")
	}
}

// ── asmResultMsg ─────────────────────────────────────────────────────────────

func TestMsg_AsmResult_EntersAsmModeOnFirstResult(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	injectMsg(es, asmResultMsg{asm: "push rbp\nmov rbp, rsp\n"})
	if !es.compositor.InAsmMode() {
		t.Error("first asm result should auto-enter asm split mode")
	}
	if !es.assembly.IsOpen() {
		t.Error("assembly widget should be open after asm result")
	}
}

func TestMsg_AsmResult_StaysInAsmModeOnRecompile(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	injectMsg(es, asmResultMsg{asm: "push rbp\n"})
	if !es.compositor.InAsmMode() {
		t.Fatal("setup: should be in asm mode after first result")
	}
	injectMsg(es, asmResultMsg{asm: "push rbp\nmov rbp, rsp\n"})
	if !es.compositor.InAsmMode() {
		t.Error("re-compile in asm mode should stay in asm mode, not close the panel")
	}
}

func TestMsg_AsmResult_Error_DoesNotEnterAsmMode(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	injectMsg(es, asmResultMsg{err: fmt.Errorf("assembler failed")})
	if es.compositor.InAsmMode() {
		t.Error("asm error should not enter asm mode")
	}
}

func TestMsg_AsmResult_EmptyOutput_DoesNotEnterAsmMode(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	injectMsg(es, asmResultMsg{asm: "   \n  \t\n"})
	if es.compositor.InAsmMode() {
		t.Error("empty asm output should not enter asm mode")
	}
}

// ── debugResultMsg (ASAN) ─────────────────────────────────────────────────────

// Regression: debugResultMsg never called timer.Stop(), so ASAN runs left the
// timer running indefinitely with the ▶ indicator after results were displayed.

func TestMsg_DebugResult_StopsTimer(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("asan")
	if !es.timer.running {
		t.Fatal("setup: timer should be running after startRun")
	}
	injectMsg(es, debugResultMsg{result: &sandbox.Result{}, err: nil})
	if es.timer.running {
		t.Error("timer should stop after debugResultMsg (ASAN)")
	}
}

func TestMsg_DebugResult_StopsTimer_OnError(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("asan")
	injectMsg(es, debugResultMsg{result: nil, err: fmt.Errorf("asan failed")})
	if es.timer.running {
		t.Error("timer should stop after debugResultMsg error path")
	}
}

func TestMsg_DebugResult_StripsAnsiFromOutput(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	raw := "\x1b[1mERROR: store to null pointer\x1b[0m"
	injectMsg(es, debugResultMsg{result: &sandbox.Result{Output: raw}, err: nil})
	// Hex viewer data should be ANSI-clean: no ESC bytes
	for i, b := range es.hexViewer.data {
		if b == 0x1b {
			t.Errorf("hex viewer data byte %d is ESC (0x1b) — ANSI not stripped", i)
			break
		}
	}
}

// ── gdbReadyMsg ───────────────────────────────────────────────────────────────

// Regression: gdbReadyMsg never called timer.Stop(), leaving the timer running
// during the entire GDB session.

func TestMsg_GdbReady_StopsTimer_OnCompileError(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("gdb")
	if !es.timer.running {
		t.Fatal("setup: timer should be running after startRun")
	}
	injectMsg(es, gdbReadyMsg{compileErr: &sandbox.Result{Error: "syntax error"}})
	if es.timer.running {
		t.Error("timer should stop after gdbReadyMsg compile error")
	}
}

// ── compileResultMsg ──────────────────────────────────────────────────────────

func TestMsg_CompileResult_StopsTimer(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.startRun("compiling")
	if !es.timer.running {
		t.Fatal("setup: timer should be running after startRun")
	}
	injectMsg(es, compileResultMsg{result: &sandbox.Result{}, err: nil})
	if es.timer.running {
		t.Error("timer should stop after compileResultMsg")
	}
}

func TestMsg_CompileResult_ClearsCompiling(t *testing.T) {
	es := newFakeExerciseScreen(t, testLesson("x", "", nil, nil))
	es.compiling = true
	injectMsg(es, compileResultMsg{result: &sandbox.Result{Error: "syntax error"}})
	if es.compiling {
		t.Error("compiling flag should be cleared after compileResultMsg")
	}
}
