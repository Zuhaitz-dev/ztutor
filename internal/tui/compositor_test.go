package tui

import (
	"testing"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

func testCompositor(t *testing.T, withFileList bool) *ExerciseCompositor {
	t.Helper()
	loc := i18n.New("en")
	lang := sandbox.GetLanguage("c")
	ed := newEditorWidget("test", "c", "vim", 60, 10)
	ed.Focus()
	fl := newFlagsWidget(80, loc)
	ar := newArgsWidget("", 80, loc)
	si := newStdinWidget("", 80, loc)
	asm := newAssemblyWidget(lang)
	out := newOutputWidget()
	di := newDiagnosticsWidget(func() int { return 1 }, 80, loc)
	ma := newMascotWidget("Mochi", "hello", 80, false)
	hu := newHintWidget([]string{"hint1"}, loc)
	te := newTestsWidget(loc)
	tr := newTriviaWidget([]string{"fun fact"})

	var flw *FileListWidget
	if withFileList {
		files := []lesson.ExerciseFile{
			{Name: "main.c", Language: "c", Editable: true, Content: "// main"},
			{Name: "utils.h", Language: "c", Editable: false, Content: "// header"},
		}
		flw = newFileListWidget(files)
		flw.SetActive(0)
	}

	ws := ParseWidgets(nil)
	c := newExerciseCompositor(ed, fl, ar, si, flw, asm, out, di, ma, hu, te, tr, ws, 80, 24, loc)
	return c
}

func TestCompositor_FocusEditor(t *testing.T) {
	c := testCompositor(t, false)
	if c.FocusID() != WidgetEditor {
		t.Errorf("initial focus = %d, want WidgetEditor", c.FocusID())
	}
	if c.InAsmMode() {
		t.Error("should not start in asm mode")
	}
}

func TestCompositor_FocusNext(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusNext()
	if c.FocusID() != WidgetFlags {
		t.Errorf("after first FocusNext: %d, want WidgetFlags", c.FocusID())
	}
	c.FocusNext()
	if c.FocusID() != WidgetArgs {
		t.Errorf("after second FocusNext: %d, want WidgetArgs", c.FocusID())
	}
	c.FocusNext()
	if c.FocusID() != WidgetStdin {
		t.Errorf("after third FocusNext: %d, want WidgetStdin", c.FocusID())
	}
	c.FocusNext()
	if c.FocusID() != WidgetEditor {
		t.Errorf("after fourth FocusNext: %d, want WidgetEditor (wrap)", c.FocusID())
	}
}

func TestCompositor_FocusNextWithFileList(t *testing.T) {
	c := testCompositor(t, true)
	for i := 0; i < 4; i++ {
		c.FocusNext()
	}
	if c.FocusID() != WidgetFileList {
		t.Errorf("FocusNext x4 = %d, want WidgetFileList", c.FocusID())
	}
	c.FocusNext()
	if c.FocusID() != WidgetEditor {
		t.Errorf("FocusNext x5 = %d, want WidgetEditor (wrap)", c.FocusID())
	}
}

func TestCompositor_FocusFileListNoOp(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusFileList()
	if c.FocusID() != WidgetEditor {
		t.Error("FocusFileList without fileList should keep editor focus")
	}
}

func TestCompositor_FocusFileListActive(t *testing.T) {
	c := testCompositor(t, true)
	c.FocusFileList()
	if c.FocusID() != WidgetFileList {
		t.Error("FocusFileList with fileList should switch to fileList")
	}
}

func TestCompositor_FocusOutput(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusOutput()
	if !c.InOutputMode() {
		t.Error("FocusOutput should set output mode")
	}
	c.FocusOutput()
	if c.InOutputMode() {
		t.Error("second FocusOutput should leave output mode")
	}
	if c.FocusID() != WidgetEditor {
		t.Error("exiting output mode should focus editor")
	}
}

func TestCompositor_FocusAsm_OpenClose(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"mov eax, 1", "ret"}, "")

	c.FocusAsm()
	if !c.InAsmMode() {
		t.Error("FocusAsm should set asm mode when assembly is open")
	}

	c.FocusAsm()
	if c.InAsmMode() {
		t.Error("second FocusAsm should leave asm mode")
	}
	if c.assembly.IsOpen() {
		t.Error("closing asm mode should close assembly")
	}
}

func TestCompositor_FocusAsm_NotOpen(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusAsm()
	if c.InAsmMode() {
		t.Error("FocusAsm should not enter asm mode when assembly is not open")
	}
}

func TestCompositor_Fullscreen(t *testing.T) {
	c := testCompositor(t, false)
	if c.InFullscreen() {
		t.Error("should not be in fullscreen initially")
	}

	c.EnterFullscreen(WidgetEditor)
	if !c.InFullscreen() {
		t.Error("should be in fullscreen after EnterFullscreen")
	}
	if c.FullscreenID() != WidgetEditor {
		t.Errorf("fullscreen ID = %d, want WidgetEditor", c.FullscreenID())
	}

	c.ExitFullscreen()
	if c.InFullscreen() {
		t.Error("should not be in fullscreen after ExitFullscreen")
	}
}

func TestCompositor_FullscreenMultiple(t *testing.T) {
	c := testCompositor(t, false)
	c.EnterFullscreen(WidgetEditor)
	if c.FullscreenID() != WidgetEditor {
		t.Error("should enter editor fullscreen")
	}

	c.EnterFullscreen(WidgetOutput)
	if c.FullscreenID() != WidgetOutput {
		t.Error("should switch to output fullscreen")
	}

	c.ExitFullscreen()
	if c.FullscreenID() != 0 {
		t.Error("should reset fullscreen on exit")
	}
}

func TestCompositor_FocusSpecificWidgets(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusFlags()
	if c.FocusID() != WidgetFlags {
		t.Error("FocusFlags failed")
	}
	c.FocusArgs()
	if c.FocusID() != WidgetArgs {
		t.Error("FocusArgs failed")
	}
	c.FocusStdin()
	if c.FocusID() != WidgetStdin {
		t.Error("FocusStdin failed")
	}
	c.FocusEditor()
	if c.FocusID() != WidgetEditor {
		t.Error("FocusEditor failed")
	}
}

func TestCompositor_RouteKey_EditorReturnsNil(t *testing.T) {
	c := testCompositor(t, false)
	cmd := c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if cmd != nil {
		t.Error("RouteKey should return nil for editor focus (caller handles)")
	}
}

func TestCompositor_RouteKey_EscToEditor(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusFlags()
	cmd := c.RouteKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("Esc should not produce a command")
	}
	if c.FocusID() != WidgetEditor {
		t.Error("Esc should return focus to editor")
	}
}

func TestCompositor_RouteKey_AsmModeKeys(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"line1", "line2", "line3", "line4", "line5"}, "")
	c.FocusAsm()

	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'g'}})
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})

	cmd := c.RouteKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("Esc in asm mode should not produce command")
	}
	if c.InAsmMode() {
		t.Error("Esc should close asm mode")
	}
}

func TestCompositor_RouteKey_OutputModeKeys(t *testing.T) {
	c := testCompositor(t, false)
	c.output.SetContent("line1\nline2\nline3")
	c.FocusOutput()

	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	cmd := c.RouteKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("Esc in output mode should not produce command")
	}
	if c.InOutputMode() {
		t.Error("Esc should leave output mode")
	}
}

func TestCompositor_Fullscreen_EscExits(t *testing.T) {
	c := testCompositor(t, false)
	c.EnterFullscreen(WidgetEditor)

	cmd := c.RouteKey(tea.KeyMsg{Type: tea.KeyEsc})
	if cmd != nil {
		t.Error("Esc in fullscreen should not produce command")
	}
	if c.InFullscreen() {
		t.Error("Esc should exit fullscreen")
	}
}

func TestCompositor_Fullscreen_AsmScroll(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"a", "b", "c", "d", "e"}, "")
	c.EnterFullscreen(WidgetAssembly)

	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})

	if !c.InFullscreen() {
		t.Error("should still be in fullscreen after scroll")
	}
}

func TestCompositor_Fullscreen_OutputScroll(t *testing.T) {
	c := testCompositor(t, false)
	c.output.SetContent("a\nb\nc")
	c.EnterFullscreen(WidgetOutput)

	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})

	if !c.InFullscreen() {
		t.Error("should still be in fullscreen after scroll")
	}
}

func TestCompositor_SetRunning(t *testing.T) {
	c := testCompositor(t, false)
	if c.InAsmMode() {
		t.Error("should not start in asm mode")
	}
	c.SetRunning(true)
	c.SetRunning(false)
}

func TestCompositor_FocusedWidget(t *testing.T) {
	c := testCompositor(t, false)
	w := c.FocusedWidget()
	if w == nil {
		t.Error("FocusedWidget should return editor")
	}
	if w.ID() != WidgetEditor {
		t.Errorf("FocusedWidget ID = %d, want WidgetEditor", w.ID())
	}
}

func TestCompositor_FocusOutputFromAsmCleansUp(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"line"}, "")
	c.FocusAsm()
	if !c.InAsmMode() {
		t.Fatal("should be in asm mode")
	}
	c.FocusOutput()
	if c.InAsmMode() {
		t.Error("FocusOutput should exit asm mode")
	}
	if c.assembly.IsOpen() {
		t.Error("FocusOutput from asm should close assembly")
	}
	if !c.InOutputMode() {
		t.Error("FocusOutput from asm should enter output mode")
	}
}

func TestCompositor_FocusAsmFromOutputCleansUp(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusOutput()
	if !c.InOutputMode() {
		t.Fatal("should be in output mode")
	}
	c.assembly.Open([]string{"line"}, "")
	c.FocusAsm()
	if c.InOutputMode() {
		t.Error("FocusAsm should exit output mode")
	}
	if !c.InAsmMode() {
		t.Error("FocusAsm from output should enter asm mode")
	}
}

func TestCompositor_FocusNextEmptyPanels(t *testing.T) {
	c := testCompositor(t, false)
	// Clear panels — should not panic
	c.panels = nil
	c.FocusNext()
}

func TestCompositor_RouteKey_BackAltFromInputs(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusFlags()
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyEsc})
	if c.FocusID() != WidgetEditor {
		t.Error("Esc from flags should return to editor")
	}
	c.FocusStdin()
	_ = c.RouteKey(tea.KeyMsg{Type: tea.KeyEsc})
	if c.FocusID() != WidgetEditor {
		t.Error("Esc from stdin should return to editor")
	}
}

// ── Validate ─────────────────────────────────────────────────────────────────

func TestCompositor_Validate_InitialState(t *testing.T) {
	c := testCompositor(t, false)
	issues := c.Validate()
	if len(issues) != 0 {
		t.Errorf("initial state should be valid, got: %v", issues)
	}
}

func TestCompositor_Validate_AsmFullscreenMutual(t *testing.T) {
	// In the refactored layout model (single layoutKind field), entering
	// fullscreen from asm mode is a coherent state transition — no conflict.
	c := testCompositor(t, false)
	c.assembly.Open([]string{"line"}, "")
	c.FocusAsm()
	c.EnterFullscreen(WidgetEditor)
	issues := c.Validate()
	if len(issues) != 0 {
		t.Errorf("entering fullscreen from asm mode should be valid, got: %v", issues)
	}
	if !c.InFullscreen() {
		t.Error("should be in fullscreen after EnterFullscreen")
	}
	if c.InAsmMode() {
		t.Error("should NOT be in asm mode after EnterFullscreen (single layout)")
	}
}

func TestCompositor_Validate_AsmWithoutAssembly(t *testing.T) {
	c := testCompositor(t, false)
	c.layout = lkAsmSplit
	issues := c.Validate()
	if len(issues) == 0 {
		t.Error("asm layout without open assembly should produce validation errors")
	}
}

func TestCompositor_Validate_NormalState(t *testing.T) {
	c := testCompositor(t, false)
	c.FocusNext()
	issues := c.Validate()
	if len(issues) != 0 {
		t.Errorf("normal state after FocusNext should be valid, got: %v", issues)
	}
}

// ── Mode transition matrix ───────────────────────────────────────────────────

func TestCompositor_ModeTransitions(t *testing.T) {
	// Verify every Focus method produces a valid state from every mode.
	// The Validate() method should return empty after each transition.
	c := testCompositor(t, true)
	c.assembly.Open([]string{"line 1", "line 2", "line 3"}, "")

	transitions := []struct {
		name string
		fn   func()
	}{
		{"FocusEditor", c.FocusEditor},
		{"FocusFlags", c.FocusFlags},
		{"FocusArgs", c.FocusArgs},
		{"FocusStdin", c.FocusStdin},
		{"FocusFileList", c.FocusFileList},
		{"FocusOutput", c.FocusOutput},
		{"FocusAsm", c.FocusAsm},
		{"FocusNext", c.FocusNext},
	}

	for _, from := range transitions {
		t.Run("from "+from.name, func(t *testing.T) {
			for _, to := range transitions {
				// Reset to known state
				c.layout = lkNormal
				c.FocusEditor()
				c.assembly.Close()

				// Apply the "from" transition
				from.fn()
				if c.InAsmMode() && !c.assembly.IsOpen() {
					c.assembly.Open([]string{"line"}, "")
				}

				// Apply the "to" transition
				to.fn()

				// Validate
				issues := c.Validate()
				if len(issues) != 0 {
					t.Errorf("%s -> %s: invalid state: %v", from.name, to.name, issues)
				}
			}
		})
	}
}

// ── Rapid toggle stress ──────────────────────────────────────────────────────

func TestCompositor_RapidAsmToggle(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"line 1"}, "")

	for i := 0; i < 100; i++ {
		c.FocusAsm()
		issues := c.Validate()
		if len(issues) != 0 {
			t.Fatalf("iteration %d after FocusAsm: %v", i, issues)
		}
		c.FocusOutput()
		issues = c.Validate()
		if len(issues) != 0 {
			t.Fatalf("iteration %d after FocusOutput: %v", i, issues)
		}
		c.FocusEditor()
		issues = c.Validate()
		if len(issues) != 0 {
			t.Fatalf("iteration %d after FocusEditor: %v", i, issues)
		}
	}
}

func TestCompositor_RapidFullscreenToggle(t *testing.T) {
	c := testCompositor(t, false)

	for i := 0; i < 100; i++ {
		c.EnterFullscreen(WidgetEditor)
		issues := c.Validate()
		if len(issues) != 0 {
			t.Fatalf("iteration %d enter fullscreen: %v", i, issues)
		}
		c.ExitFullscreen()
		issues = c.Validate()
		if len(issues) != 0 {
			t.Fatalf("iteration %d exit fullscreen: %v", i, issues)
		}
	}
}

func TestCompositor_RapidFocusCycle(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"line"}, "")

	for i := 0; i < 500; i++ {
		c.FocusNext()
		issues := c.Validate()
		if len(issues) != 0 {
			t.Fatalf("iteration %d FocusNext: %v", i, issues)
		}
	}
}

// ── regression tests ──────────────────────────────────────────────────────────

// Regression: pressing the fullscreen key twice (e.g. F4 → F4 while already
// fullscreen) overwrote prevLayout with lkFullscreen, requiring two Esc presses
// to exit instead of one.
func TestCompositor_EnterFullscreen_DoublePressRequiresOneEsc(t *testing.T) {
	c := testCompositor(t, false)

	c.EnterFullscreen(WidgetEditor)
	if !c.InFullscreen() {
		t.Fatal("setup: should be in fullscreen after first EnterFullscreen")
	}

	// Second press — the bug: prevLayout would be clobbered with lkFullscreen
	c.EnterFullscreen(WidgetEditor)
	if !c.InFullscreen() {
		t.Error("should still be in fullscreen after second EnterFullscreen")
	}

	c.ExitFullscreen()
	if c.InFullscreen() {
		t.Error("one Esc should exit fullscreen, not require a second press")
	}
}

// Regression: switching between fullscreen widgets (F4 → F5) and then pressing
// Esc should restore the pre-fullscreen layout, not stay in fullscreen.
func TestCompositor_EnterFullscreen_SwitchWidgetPreservesPrevLayout(t *testing.T) {
	c := testCompositor(t, false)
	c.assembly.Open([]string{"line"}, "")
	c.FocusAsm()
	if !c.InAsmMode() {
		t.Fatal("setup: should be in asm split")
	}

	c.EnterFullscreen(WidgetEditor)   // from asm split → prevLayout = lkAsmSplit
	c.EnterFullscreen(WidgetAssembly) // already fullscreen → prevLayout must NOT change

	if c.FullscreenID() != WidgetAssembly {
		t.Error("should have switched to assembly fullscreen")
	}

	c.ExitFullscreen()
	if c.InFullscreen() {
		t.Error("should exit fullscreen after one Esc")
	}
	if !c.InAsmMode() {
		t.Error("should restore asm split layout, not lkNormal")
	}
}

// FocusEditorInPlace moves focus without touching layout — used when switching
// files while in fullscreen editor.
func TestCompositor_FocusEditorInPlace_PreservesFullscreen(t *testing.T) {
	c := testCompositor(t, false)
	c.EnterFullscreen(WidgetEditor)

	c.FocusEditorInPlace()

	if !c.InFullscreen() {
		t.Error("FocusEditorInPlace should not exit fullscreen")
	}
	if c.FullscreenID() != WidgetEditor {
		t.Errorf("fullscreen widget = %d, want WidgetEditor", c.FullscreenID())
	}
	if c.FocusID() != WidgetEditor {
		t.Error("FocusEditorInPlace should focus the editor")
	}
}

// FocusEditor (the old path) resets layout — this test documents the contrast
// so readers understand why FocusEditorInPlace is needed for file switching.
func TestCompositor_FocusEditor_ExitsFullscreen(t *testing.T) {
	c := testCompositor(t, false)
	c.EnterFullscreen(WidgetEditor)

	c.FocusEditor()

	if c.InFullscreen() {
		t.Error("FocusEditor should reset layout and exit fullscreen")
	}
}
