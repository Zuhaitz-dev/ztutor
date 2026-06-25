package tui

import (
	"strings"

	"ztutor/internal/i18n"

	tea "github.com/charmbracelet/bubbletea"
)

// layoutKind is the single source of truth for the current screen layout.
// It replaces the old compositorMode + fullscreen WidgetID pair, which could
// both be set simultaneously and cause rendering conflicts.
type layoutKind int

const (
	lkNormal     layoutKind = iota // normal editor view
	lkAsmSplit                     // side-by-side editor + assembly
	lkOutput                       // output panel focused
	lkFullscreen                   // one widget fills the terminal
)

// ExerciseCompositor manages widget lifecycle, focus state, key routing, and
// screen layout for the exercise screen.
//
// Layout: lkNormal / lkAsmSplit / lkOutput / lkFullscreen(fscreenID). Only
// one layout is active at a time — no more mode+fullscreen conflict.
//
// Focus: focusID tracks which widget receives key input, independent of layout.
// In fullscreen editor the file list can still be focused and navigated.
//
// Overlays: they always handle their own scroll keys first, before any layout
// logic, so overlay scroll works in every layout state.
type ExerciseCompositor struct {
	panels []Widget

	editor   *EditorWidget
	flags    *FlagsWidget
	args     *ArgsWidget
	stdin    *StdinWidget
	fileList *FileListWidget
	assembly *AssemblyWidget
	output   *OutputWidget
	diag     *DiagnosticsWidget
	mascot   *MascotWidget
	hint     *HintWidget
	tests    *TestsWidget
	trivia   *TriviaWidget

	focusID    WidgetID
	layout     layoutKind // current screen layout
	fscreenID  WidgetID   // which widget is fullscreen (only when layout == lkFullscreen)
	prevLayout layoutKind // layout to restore when exiting fullscreen
	running    bool       // compile/run in progress
	width      int
	height     int
	editorH    int
	outputH    int // actual output panel height, set each frame by the layout renderer
	loc        *i18n.Locale
}

func newExerciseCompositor(
	editor *EditorWidget,
	flags *FlagsWidget,
	args *ArgsWidget,
	stdin *StdinWidget,
	fileList *FileListWidget,
	assembly *AssemblyWidget,
	output *OutputWidget,
	diag *DiagnosticsWidget,
	mascot *MascotWidget,
	hint *HintWidget,
	tests *TestsWidget,
	trivia *TriviaWidget,
	ws EnabledWidgets,
	width, height int,
	loc *i18n.Locale,
) *ExerciseCompositor {
	c := &ExerciseCompositor{
		editor:   editor,
		flags:    flags,
		args:     args,
		stdin:    stdin,
		fileList: fileList,
		assembly: assembly,
		output:   output,
		diag:     diag,
		mascot:   mascot,
		hint:     hint,
		tests:    tests,
		trivia:   trivia,
		focusID:  WidgetEditor,
		width:    width,
		height:   height,
		loc:      loc,
	}
	c.panels = c.buildPanels(ws)
	editor.Focus()
	return c
}

func (c *ExerciseCompositor) SetLocale(loc *i18n.Locale) { c.loc = loc }

func (c *ExerciseCompositor) FocusID() WidgetID      { return c.focusID }
func (c *ExerciseCompositor) InAsmMode() bool        { return c.layout == lkAsmSplit }
func (c *ExerciseCompositor) InOutputMode() bool     { return c.layout == lkOutput }
func (c *ExerciseCompositor) SetRunning(v bool)      { c.running = v }
func (c *ExerciseCompositor) FullscreenID() WidgetID { return c.fscreenID }
func (c *ExerciseCompositor) InFullscreen() bool     { return c.layout == lkFullscreen }

func (c *ExerciseCompositor) EnterFullscreen(id WidgetID) {
	// Only save prevLayout when we are not already in fullscreen; otherwise we
	// would clobber the real pre-fullscreen layout with lkFullscreen itself,
	// which would require two Esc presses to exit (once to "restore fullscreen",
	// once to actually exit).
	if c.layout != lkFullscreen {
		c.prevLayout = c.layout
	}
	c.layout = lkFullscreen
	c.fscreenID = id
}

func (c *ExerciseCompositor) ExitFullscreen() {
	c.layout = c.prevLayout
	c.fscreenID = 0
	c.prevLayout = lkNormal
}

func (c *ExerciseCompositor) Validate() []string {
	var issues []string
	if c.layout == lkAsmSplit && !c.assembly.IsOpen() {
		issues = append(issues, "asm layout active but assembly is not open")
	}
	return issues
}

func (c *ExerciseCompositor) FocusedWidget() Widget {
	switch c.focusID {
	case WidgetEditor:
		return c.editor
	case WidgetFlags:
		return c.flags
	case WidgetArgs:
		return c.args
	case WidgetStdin:
		return c.stdin
	case WidgetFileList:
		return c.fileList
	case WidgetAssembly:
		return c.assembly
	}
	return nil
}

func (c *ExerciseCompositor) buildPanels(ws EnabledWidgets) []Widget {
	p := []Widget{c.editor}
	if ws.Has(WidgetFlags) {
		p = append(p, c.flags)
	}
	if ws.Has(WidgetArgs) {
		p = append(p, c.args)
	}
	if ws.Has(WidgetStdin) {
		p = append(p, c.stdin)
	}
	if c.fileList != nil && ws.Has(WidgetFileList) {
		p = append(p, c.fileList)
	}
	return p
}

func (c *ExerciseCompositor) FocusNext() {
	if c.layout == lkAsmSplit {
		c.assembly.Close()
		c.FocusEditor()
		return
	}
	// Cycling to input panels requires normal layout (inputs not visible in fullscreen).
	c.resetLayout()
	if len(c.panels) == 0 {
		return
	}
	idx := c.panelIndex()
	if idx < 0 {
		c.FocusEditor()
		return
	}
	next := (idx + 1) % len(c.panels)
	c.setFocusWidget(c.panels[next])
}

// resetLayout clears fullscreen and any special layout, returning to lkNormal.
func (c *ExerciseCompositor) resetLayout() {
	c.layout = lkNormal
	c.fscreenID = 0
	c.prevLayout = lkNormal
}

func (c *ExerciseCompositor) FocusEditor() {
	c.resetLayout()
	c.setFocusWidget(c.editor)
}

func (c *ExerciseCompositor) FocusFlags() {
	c.resetLayout()
	c.setFocusWidget(c.flags)
}

func (c *ExerciseCompositor) FocusArgs() {
	c.resetLayout()
	c.setFocusWidget(c.args)
}

func (c *ExerciseCompositor) FocusStdin() {
	c.resetLayout()
	c.setFocusWidget(c.stdin)
}

// FocusFileList focuses the file list without changing the layout, so it works
// inside fullscreen editor (file navigation without exiting fullscreen).
func (c *ExerciseCompositor) FocusFileList() {
	if c.fileList != nil && c.fileList.Available() && c.layout != lkAsmSplit {
		c.setFocusWidget(c.fileList)
	}
}

// FocusEditorInPlace moves focus to the editor without changing the layout.
// Use this when the editor should be focused but the current layout (e.g.
// fullscreen) must be preserved — for example after a file switch in multifile.
func (c *ExerciseCompositor) FocusEditorInPlace() {
	c.setFocusWidget(c.editor)
}

func (c *ExerciseCompositor) FocusOutput() {
	if c.layout == lkAsmSplit {
		if c.assembly.IsOpen() {
			c.assembly.Close()
		}
	}
	if c.layout == lkOutput {
		c.FocusEditor()
		return
	}
	c.blurAll()
	c.layout = lkOutput
	c.fscreenID = 0
	c.prevLayout = lkNormal
	c.focusID = WidgetOutput
}

func (c *ExerciseCompositor) FocusAsm() {
	if c.layout == lkOutput {
		c.layout = lkNormal
	}
	if c.layout == lkAsmSplit {
		if c.assembly.IsOpen() {
			c.assembly.Close()
		}
		c.FocusEditor()
	} else if c.assembly.IsOpen() {
		c.blurAll()
		c.layout = lkAsmSplit
		c.fscreenID = 0
		c.prevLayout = lkNormal
		c.focusID = WidgetAssembly
		c.assembly.Focus()
	}
}

func (c *ExerciseCompositor) panelIndex() int {
	for i, w := range c.panels {
		if w.ID() == c.focusID {
			return i
		}
	}
	return -1
}

func (c *ExerciseCompositor) setFocusWidget(w Widget) {
	c.blurAll()
	if w != nil {
		w.Focus()
		c.focusID = w.ID()
	}
	// Layout is managed by callers; setFocusWidget only changes focus.
}

func (c *ExerciseCompositor) blurAll() {
	c.editor.Blur()
	c.flags.Blur()
	c.args.Blur()
	c.stdin.Blur()
	if c.fileList != nil {
		c.fileList.Blur()
	}
	if c.assembly.IsOpen() {
		c.assembly.Blur()
	}
}

type overlayScroller interface {
	Focused() bool
	ScrollDown(int)
	ScrollUp()
	ScrollTop()
	ScrollBottom(int)
}

func (c *ExerciseCompositor) RouteKey(msg tea.KeyMsg) tea.Cmd {
	return c.routeKeyInternal(msg, nil)
}

func (c *ExerciseCompositor) RouteKeyWithOverlay(msg tea.KeyMsg, overlay overlayScroller) tea.Cmd {
	return c.routeKeyInternal(msg, overlay)
}

func (c *ExerciseCompositor) routeKeyInternal(msg tea.KeyMsg, overlay overlayScroller) tea.Cmd {
	// 1. Overlay always intercepts its own scroll/close keys first, in any layout.
	//    This fixes the bug where j/k did not scroll the keybindings overlay while
	//    in fullscreen assembly or output.
	if overlay != nil && overlay.Focused() {
		switch msg.String() {
		case KeyDownVim, KeyDown:
			overlay.ScrollDown(c.height)
			return nil
		case KeyUpVim, KeyUp:
			overlay.ScrollUp()
			return nil
		case KeyScrollTop:
			overlay.ScrollTop()
			return nil
		case KeyScrollBot:
			overlay.ScrollBottom(c.height)
			return nil
		case KeyBackAlt:
			return nil
		}
	}

	// 2. Fullscreen: Esc exits and restores previous layout; assembly/output
	//    fullscreen handle their scroll keys and swallow everything else.
	//    Fullscreen editor falls through to normal focusID routing so the
	//    file list and editor work exactly as in non-fullscreen mode.
	if c.layout == lkFullscreen {
		if msg.String() == KeyBackAlt {
			c.ExitFullscreen()
			return nil
		}
		switch c.fscreenID {
		case WidgetAssembly:
			switch msg.String() {
			case KeyDownVim, KeyDown:
				c.assembly.ScrollDown(c.height)
			case KeyUpVim, KeyUp:
				c.assembly.ScrollUp()
			case KeyScrollTop:
				c.assembly.ScrollTop()
			case KeyScrollBot:
				c.assembly.ScrollBottom(c.height)
			}
			return nil
		case WidgetOutput:
			switch msg.String() {
			case KeyDownVim, KeyDown:
				c.output.ScrollDown(c.height)
			case KeyUpVim, KeyUp:
				c.output.ScrollUp()
			case KeyScrollTop:
				c.output.ScrollTop()
			case KeyScrollBot:
				c.output.ScrollBottom(c.height)
			}
			return nil
		}
		// lkFullscreen + WidgetEditor: fall through to normal focusID routing.
	}

	// 3. Layout-specific routing for split/output modes.
	switch c.layout {
	case lkAsmSplit:
		return c.routeAsmKey(msg)
	case lkOutput:
		return c.routeOutputKey(msg)
	}

	// 4. Normal focusID routing.
	switch c.focusID {
	case WidgetEditor:
		return nil
	case WidgetFileList:
		return c.routeFileListKey(msg)
	case WidgetFlags, WidgetArgs, WidgetStdin:
		if msg.String() == KeyBackAlt {
			c.FocusEditor()
			return nil
		}
		return c.updateInPlace(c.FocusedWidget(), msg)
	}
	if msg.String() == KeyBackAlt {
		c.FocusEditor()
		return nil
	}
	return nil
}

func (c *ExerciseCompositor) updateInPlace(w Widget, msg tea.Msg) tea.Cmd {
	switch t := w.(type) {
	case *FlagsWidget:
		return t.UpdateInPlace(msg)
	case *ArgsWidget:
		return t.UpdateInPlace(msg)
	case *StdinWidget:
		return t.UpdateInPlace(msg)
	default:
		_, cmd := w.Update(msg)
		return cmd
	}
}

func (c *ExerciseCompositor) routeAsmKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case KeyBackAlt, KeyBack:
		c.assembly.Close()
		c.FocusEditor()
	case KeyDownVim, KeyDown:
		c.assembly.ScrollDown(c.editorH)
	case KeyUpVim, KeyUp:
		c.assembly.ScrollUp()
	case KeyScrollTop:
		c.assembly.ScrollTop()
	case KeyScrollBot:
		c.assembly.ScrollBottom(c.editorH)
	case "[", "left":
		c.assembly.ShrinkEditor()
	case "]", "right":
		c.assembly.GrowEditor()
	}
	return nil
}

func (c *ExerciseCompositor) routeOutputKey(msg tea.KeyMsg) tea.Cmd {
	h := c.outputH
	if h <= 0 {
		h = c.height / 3
	}
	switch msg.String() {
	case KeyBackAlt, KeyBack:
		c.FocusEditor()
	case KeyDownVim, KeyDown:
		c.output.ScrollDown(h)
	case KeyUpVim, KeyUp:
		c.output.ScrollUp()
	case KeyScrollTop:
		c.output.ScrollTop()
	case KeyScrollBot:
		c.output.ScrollBottom(h)
	}
	return nil
}

func (c *ExerciseCompositor) routeFileListKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case KeyBackAlt:
		c.FocusEditor()
	case KeyInputs:
		c.FocusEditor()
	case "up", "k":
		c.fileList.MoveUp()
	case "down", "j":
		c.fileList.MoveDown()
	}
	return nil
}

func (c *ExerciseCompositor) SetSize(width, height int) {
	c.width, c.height = width, height
}

// SetOutputH records the actual rendered height of the output panel so that
// scroll clamping uses the panel height rather than the full terminal height.
// Call this once per frame from the output panel's layout closure.
func (c *ExerciseCompositor) SetOutputH(h int) { c.outputH = h }

func (c *ExerciseCompositor) HelpBar(passed, running, hasFileList bool, loc *i18n.Locale) string {
	return c.helpBar(passed, running, hasFileList, loc)
}

func (c *ExerciseCompositor) helpBar(passed, running, hasFileList bool, loc *i18n.Locale) string {
	var b strings.Builder
	var row1, row2 []HelpAction

	switch {
	case running:
		row1 = []HelpAction{HA(ActionSend), HA(ActionKill), HA(ActionExerciseBack)}
	case c.layout == lkAsmSplit:
		row1 = []HelpAction{HA(ActionScroll), HA(ActionTopEnd), HA(ActionAsmResize), HA(ActionAsmAnnotate), HA(ActionAsmHide), HA(ActionEscBack)}
	case c.layout == lkOutput:
		row1 = []HelpAction{HA(ActionScroll), HA(ActionTopEnd), HA(ActionOutputResize), HA(ActionEscBack)}
	case c.focusID == WidgetFileList:
		row1 = []HelpAction{HA(ActionFileListMove), HA(ActionFileListSelect), HA(ActionEscBack)}
	case passed:
		row1 = []HelpAction{HA(ActionNext), HA(ActionExerciseBack)}
	default:
		row1 = []HelpAction{HA(ActionRun), HA(ActionInteractive), HA(ActionASAN), HA(ActionGDB), HA(ActionExerciseBack)}
		row2 = []HelpAction{HA(ActionAssembly), HA(ActionInputs), HA(ActionOutput), HA(ActionMochi), HA(ActionHintStart, 0), HA(ActionTrivia)}
		if hasFileList {
			row2 = append(row2, HA(ActionFileList))
		}
	}

	b.WriteString(actionHelpBar(loc, row1...))
	if len(row2) > 0 {
		b.WriteString("\n")
		b.WriteString(actionHelpBar(loc, row2...))
	}
	return b.String()
}
