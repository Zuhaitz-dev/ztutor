package tui

import (
	"os"
	"strings"
	"time"

	editormod "ztutor/internal/editor"
	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/logutil"
	"ztutor/internal/sandbox"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type programOutputMsg struct{ text string }
type programDoneMsg struct{ code int }
type diagTimerMsg struct{ version int }
type diagResultMsg struct {
	diags   []sandbox.Diagnostic
	version int
}

type compileResultMsg struct {
	result *sandbox.Result
	err    error
}

type testRunResultMsg struct {
	compileResult *sandbox.Result
	testResults   []sandbox.TestResult
	err           error
}

type debugResultMsg struct {
	result *sandbox.Result
	err    error
}

type gdbReadyMsg struct {
	build      *sandbox.DebugBuild
	compileErr *sandbox.Result
}

type interactiveReadyMsg struct {
	build      *sandbox.DebugBuild
	compileErr *sandbox.Result
}

type asmResultMsg struct {
	asm string
	err error
}

type runMode int

const (
	runModeNone        runMode = iota
	runModeInteractive         // Ctrl+E: textinput-based interactive mode
	runModeGDB                 // Ctrl+G: raw key forwarding to GDB
)

type ExerciseScreen struct {
	lesson   lesson.Lesson
	lang     sandbox.Language
	executor sandbox.Executor
	keymap   string

	activeFiles []ActiveFile
	activeIdx   int

	compositor *ExerciseCompositor

	editor           *EditorWidget
	flags            *FlagsWidget
	args             *ArgsWidget
	stdin            *StdinWidget
	fileList         *FileListWidget
	assembly         *AssemblyWidget
	output           *OutputWidget
	hint             *HintWidget
	mascot           *MascotWidget
	diag             *DiagnosticsWidget
	tests            *TestsWidget
	trivia           *TriviaWidget
	progress         *ProgressWidget
	timer            *TimerWidget
	memory           *MemoryWidget
	reference        *ReferenceWidget
	kbOverlay        *KeybindingsOverlay
	streak           *StreakWidget
	console          *ConsoleWidget
	hexViewer        *HexViewerWidget
	structInsp       *StructInspectorWidget
	mascotWelcomeKey string
	passed           bool

	running       bool
	runMode       runMode
	interWrite    func([]byte) error
	interKill     func()
	interCh       <-chan sandbox.InteractiveEvent
	interBuild    *sandbox.DebugBuild
	gdbInitPath   string // temp gdb init file, cleaned up on GDB exit
	runInput      textinput.Model
	liveOutput    string
	programOutput string

	diagVersion int

	previousStars   int
	attempts        int
	lastHasWarnings bool
	earnedStars     int

	compiling  bool
	runStarted time.Time

	outputSplit int // output panel flex weight (2–8, default 3); editor gets 10-outputSplit

	sized
	loc *i18n.Locale
}

var (
	exHeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorAccent))

	exOutputStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)

	exSuccessStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color(ColorSuccess))

	exErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorError))

	flagsLabelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color(ColorDim))

	flagsLabelFocusedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color(ColorAccent))
)

func NewExerciseScreen(l lesson.Lesson, lang sandbox.Language, executor sandbox.Executor, width, height int, keymap string, previousStars, streak int, loc *i18n.Locale, showMascot, showTimer bool) *ExerciseScreen {
	if loc == nil {
		loc = i18n.New("en")
	}
	editorH := height / 2

	var activeFiles []ActiveFile
	var fileList *FileListWidget
	var initialContent, hlLang string
	activeLang := lang

	if len(l.Files) > 0 {
		activeFiles = make([]ActiveFile, len(l.Files))
		for i, f := range l.Files {
			fl := sandbox.GetLanguage(f.Language)
			activeFiles[i] = ActiveFile{File: f, Content: f.Content, Lang: fl}
		}
		initialIdx := 0
		for i, f := range l.Files {
			if f.Editable {
				initialIdx = i
				break
			}
		}
		af := activeFiles[initialIdx]
		initialContent = af.Content
		if af.Lang != nil {
			hlLang = af.Lang.Name()
			activeLang = af.Lang
		} else {
			hlLang = af.File.Language
		}
		if len(l.Files) > 1 {
			fileList = newFileListWidget(l.Files)
			fileList.SetActive(initialIdx)
		}
	} else {
		initialContent = l.Exercise
		hlLang = l.SyntaxHighlighting
	}
	if hlLang == "" {
		hlLang = "c"
	}

	edW := editorWidth(width, false)
	if fileList != nil {
		edW -= fileListWidth + 1
	}
	ed := newEditorWidget(initialContent, hlLang, keymap, edW, editorH)

	firstArgs, firstStdin := "", ""
	if len(l.Tests) > 0 {
		firstArgs = l.Tests[0].Args
		firstStdin = l.Tests[0].Stdin
	}

	runIn := textinput.New()
	runIn.Placeholder = loc.T("exercise.placeholder.stdin")
	runIn.CharLimit = 500
	runIn.Prompt = ""

	welcomeKey := "exercise.mochi.welcome"
	if lang != nil {
		switch lang.Name() {
		case "python":
			welcomeKey = "exercise.mochi.welcome_python"
		case "c", "cpp":
			welcomeKey = "exercise.mochi.welcome"
		default:
			welcomeKey = "exercise.mochi.welcome_default"
		}
	}
	mascot := newMascotWidget("Mochi", loc.T(welcomeKey), width, false)
	if !showMascot {
		mascot.ToggleHidden()
	}

	curLineFn := func() int { return ed.Row() + 1 }

	ws := ParseWidgets(l.EnabledWidgets)

	fl := newFlagsWidget(width, loc)
	ar := newArgsWidget(firstArgs, width, loc)
	si := newStdinWidget(firstStdin, width, loc)
	asm := newAssemblyWidget(activeLang)
	out := newOutputWidget()
	hu := newHintWidget(l.Hints, loc)
	di := newDiagnosticsWidget(curLineFn, width, loc)
	tw := newTestsWidget(loc)
	tr := newTriviaWidget(l.Trivia)
	prog := newProgressWidget()
	timer := newTimerWidget()
	if !showTimer {
		timer.Toggle()
	}
	mem := newMemoryWidget(loc)
	ref := newReferenceWidget(l.References, loc)
	kbo := newKeybindingsOverlay(loc)
	strk := newStreakWidget(streak, loc)
	csl := newConsoleWidget(loc)
	hex := newHexViewerWidget(loc)
	sinsp := newStructInspectorWidget(loc)
	es := &ExerciseScreen{
		lesson:           l,
		lang:             activeLang,
		executor:         executor,
		keymap:           keymap,
		editor:           ed,
		fileList:         fileList,
		activeFiles:      activeFiles,
		flags:            fl,
		args:             ar,
		stdin:            si,
		assembly:         asm,
		output:           out,
		hint:             hu,
		mascot:           mascot,
		diag:             di,
		tests:            tw,
		trivia:           tr,
		progress:         prog,
		timer:            timer,
		memory:           mem,
		reference:        ref,
		kbOverlay:        kbo,
		streak:           strk,
		console:          csl,
		hexViewer:        hex,
		structInsp:       sinsp,
		mascotWelcomeKey: welcomeKey,
		runInput:         runIn,
		previousStars:    previousStars,
		outputSplit:      3,
		sized:            sized{Width: width, Height: height},
		loc:              loc,
		compositor:       newExerciseCompositor(ed, fl, ar, si, fileList, asm, out, di, mascot, hu, tw, tr, ws, width, height, loc),
	}

	return es
}

func (es *ExerciseScreen) SetHasGamepad(v bool) { es.kbOverlay.SetHasGamepad(v) }

func editorWidth(totalW int, asmVisible bool) int {
	if asmVisible {
		// Left half minus line-number gutter
		return (totalW-1)/2 - editormod.LineNumWidth - 2
	}
	return totalW - editormod.LineNumWidth - 2
}

func flagsInputWidth(totalW int) int {
	w := totalW - 12 // "Flags: [  ] " overhead
	if w < 10 {
		w = 10
	}
	return w
}

func parseFlags(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	return strings.Fields(s)
}

func (es *ExerciseScreen) Init() tea.Cmd {
	if code := es.editor.Value(); code != "" && es.lang != nil {
		es.diagVersion = 1
		return es.syntaxCheckCmd(parseFlags(es.flags.Value()), 1)
	}
	return nil
}

func (es *ExerciseScreen) SetLocale(loc *i18n.Locale) {
	es.loc = loc
	es.flags.SetLocale(loc)
	es.args.SetLocale(loc)
	es.stdin.SetLocale(loc)
	es.hint.SetLocale(loc)
	es.diag.SetLocale(loc)
	es.tests.SetLocale(loc)
	es.compositor.SetLocale(loc)
	es.mascot.SetRTL(loc.IsRTL())
	es.mascot.SetLine(loc.T(es.mascotWelcomeKey))
	es.runInput.Placeholder = loc.T("exercise.placeholder.stdin")
}

// SetLesson refreshes locale-dependent lesson metadata without replacing the
// learner's in-progress editor state.
func (es *ExerciseScreen) SetLesson(l lesson.Lesson) {
	es.lesson = l
	es.hint.SetHints(l.Hints)
	es.trivia.SetItems(l.Trivia)
	es.reference.SetReferences(l.References)
}

func (es *ExerciseScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if es.running {
			switch es.runMode {
			case runModeGDB:
				s := msg.String()
				if s == KeyBackEditor || s == KeyQuit {
					if es.interKill != nil {
						es.interKill()
					}
					if es.compositor.InFullscreen() {
						es.compositor.ExitFullscreen()
					}
					os.Remove(es.gdbInitPath)
					es.gdbInitPath = ""
					return es, backCmd(NavigateBackToCourse{})
				}
				if s == KeyGdb {
					if es.compositor.InFullscreen() && es.compositor.FullscreenID() == WidgetOutput {
						es.compositor.ExitFullscreen()
					} else {
						es.compositor.EnterFullscreen(WidgetOutput)
					}
					return es, nil
				}
				switch msg.Type {
				case tea.KeyRunes:
					for _, r := range msg.Runes {
						_ = es.interWrite([]byte(string(r)))
					}
				case tea.KeySpace:
					_ = es.interWrite([]byte(" "))
				case tea.KeyEnter:
					_ = es.interWrite([]byte("\n"))
				case tea.KeyTab:
					_ = es.interWrite([]byte("\t"))
				case tea.KeyBackspace:
					_ = es.interWrite([]byte("\x7f"))
				case tea.KeyCtrlC:
					_ = es.interWrite([]byte("\x03"))
				case tea.KeyCtrlD:
					_ = es.interWrite([]byte("\x04"))
				case tea.KeyEscape:
					_ = es.interWrite([]byte("\x1b"))
				case tea.KeyUp, tea.KeyDown, tea.KeyLeft, tea.KeyRight:
					seq := arrowSeq(msg.Type)
					if seq != "" {
						_ = es.interWrite([]byte(seq))
					}
				}
				return es, nil

			case runModeInteractive:
				switch msg.String() {
				case KeyQuit:
					if es.interKill != nil {
						es.interKill()
					}
				case KeyBackEditor:
					if es.interKill != nil {
						es.interKill()
					}
					return es, backCmd(NavigateBackToCourse{})
				case KeySelect:
					line := es.runInput.Value()
					if es.interWrite != nil {
						if err := es.interWrite([]byte(line + "\n")); err != nil {
							logutil.Error("exercise: interactive write: %v", err)
						}
					}
					es.liveOutput += "» " + line + "\n"
					es.output.SetContent(es.liveOutput)
					es.runInput.Reset()
				default:
					var cmd tea.Cmd
					es.runInput, cmd = es.runInput.Update(msg)
					return es, cmd
				}
				return es, nil
			}
			return es, nil
		}

		switch msg.String() {
		case KeyBackEditor:
			return es, backCmd(NavigateBackToCourse{})

		case KeyInputs:
			es.compositor.FocusNext()
			return es, nil

		case KeyStdin:
			es.compositor.FocusStdin()
			return es, nil

		case KeyFileList:
			if es.fileList != nil && es.fileList.Available() {
				es.compositor.FocusFileList()
			}
			return es, nil

		case KeyRun:
			if !es.compiling {
				runLine := es.loc.T("exercise.mochi.running")
				if eggLine := codeEasterEggLine(es.lang, es.editor.Value(), es.loc); eggLine != "" {
					runLine = eggLine
				}
				es.startRun(runLine)
				es.attempts++
				if len(es.lesson.Tests) >= 2 {
					inputs := make([]sandbox.TestInput, len(es.lesson.Tests))
					for i, tc := range es.lesson.Tests {
						inputs[i] = sandbox.TestInput{
							Stdin: tc.Stdin, Args: parseFlags(tc.Args), Expected: tc.Expected,
						}
					}
					return es, es.runAllTestsCmd(parseFlags(es.flags.Value()), inputs)
				}
				return es, es.compileCmd(es.stdin.Value(), parseFlags(es.flags.Value()), parseFlags(es.args.Value()))
			}
			return es, nil

		case KeyAsan:
			if !es.compiling && es.lang != nil && es.lang.HasSanitizers() {
				es.startRun(es.loc.T("exercise.mochi.asan_start"))
				return es, es.debugCmd(es.stdin.Value(), parseFlags(es.flags.Value()), parseFlags(es.args.Value()))
			}
			return es, nil

		case KeyGdb:
			if !es.compiling && es.lang != nil && es.lang.HasDebugger() {
				es.startRun(es.loc.T("exercise.mochi.gdb_start"))
				return es, es.gdbCompileCmd(parseFlags(es.flags.Value()))
			}
			return es, nil

		case KeyAsm:
			if es.lang == nil || !es.lang.HasAssembly() {
				return es, nil
			}
			if es.compositor.InAsmMode() {
				es.assembly.Close()
				es.compositor.FocusEditor()
			} else if es.assembly.IsOpen() {
				es.compositor.FocusAsm()
			} else {
				es.mascot.Speak(es.loc.T("exercise.mochi.asm_view"), MoodFocused)
				flags := parseFlags(es.flags.Value())
				if es.assembly.Annotated() {
					flags = append(flags, "-fverbose-asm", "-g")
				}
				return es, es.asmCmd(flags)
			}
			return es, nil

		case KeyAsmAnnotate:
			if es.compositor.InAsmMode() && es.lang != nil && es.lang.HasAssembly() {
				es.assembly.ToggleAnnotated()
				flags := parseFlags(es.flags.Value())
				if es.assembly.Annotated() {
					flags = append(flags, "-fverbose-asm", "-g")
				}
				return es, es.asmCmd(flags)
			}

		case KeyOutput:
			es.compositor.FocusOutput()
			return es, nil

		case KeyInteract:
			if !es.compiling {
				es.startRun(es.loc.T("exercise.mochi.interactive"))
				return es, es.interactiveCompileCmd(parseFlags(es.flags.Value()))
			}
			return es, nil

		case KeyHintEx:
			if es.compositor.FocusID() != WidgetEditor || es.passed {
				if es.hint.Available() {
					es.hint.Next()
					es.mascot.Speak(es.loc.T("exercise.mochi.hint", es.hint.CurrentIndex()+1), MoodCurious)
				}
				return es, nil
			}

		case KeyMochi:
			if es.compositor.FocusID() != WidgetEditor || es.passed {
				es.mascot.ToggleHidden()
				val := "1"
				if es.mascot.IsHidden() {
					val = "0"
				}
				return es, func() tea.Msg { return persistSettingMsg{key: "mascot_visible", value: val} }
			}

		case KeyTrivia:
			if es.compositor.FocusID() != WidgetEditor || es.passed {
				if es.trivia.Available() {
					es.trivia.Next()
					es.mascot.Speak(es.trivia.Current(), MoodCurious)
				}
				return es, nil
			}

		case KeyHexView:
			es.hexViewer.Toggle()
			if es.memory.IsOpen() {
				lbl := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAmber)).Render(es.loc.T("exercise.result.asan_label"))
				rebuilt := lbl + "\n\n" + es.memory.View()
				if es.hexViewer.IsVisible() {
					rebuilt += es.hexViewer.View(99999)
				}
				es.output.SetContent(rebuilt)
			}
			return es, nil

		case KeyStructView:
			es.structInsp.Toggle()
			if es.structInsp.IsVisible() {
				es.structInsp.SetCode(es.editor.Value())
			}
			return es, nil

		case KeyOutputGrow:
			if es.outputSplit < 8 {
				es.outputSplit++
			}
			return es, nil

		case KeyOutputShrink:
			if es.outputSplit > 2 {
				es.outputSplit--
			}
			return es, nil

		case KeyTimer:
			es.timer.Toggle()
			val := "1"
			if !es.timer.IsVisible() {
				val = "0"
			}
			return es, func() tea.Msg { return persistSettingMsg{key: "timer_visible", value: val} }

		case KeyRef:
			es.reference.Toggle()
			return es, nil

		case KeyHelp:
			es.kbOverlay.Toggle()
			return es, nil

		case KeyFullEditor:
			es.compositor.EnterFullscreen(WidgetEditor)
			return es, nil
		case KeyFullAssembly:
			if es.assembly.IsOpen() {
				es.compositor.EnterFullscreen(WidgetAssembly)
			}
			return es, nil
		case KeyFullOutput:
			es.compositor.EnterFullscreen(WidgetOutput)
			return es, nil
		}

		if es.kbOverlay.IsVisible() {
			switch msg.String() {
			case KeyBackAlt, KeyHelp:
				es.kbOverlay.Hide()
				return es, nil
			}
		}

		var routeCmd tea.Cmd
		if es.kbOverlay.IsVisible() {
			routeCmd = es.compositor.RouteKeyWithOverlay(msg, es.kbOverlay)
		} else {
			routeCmd = es.compositor.RouteKey(msg)
		}
		if routeCmd != nil {
			return es, routeCmd
		}

		if es.compositor.FocusID() == WidgetEditor {
			if msg.String() == KeySelect && es.passed {
				stars := es.earnedStars
				id := es.lesson.ID
				return es, func() tea.Msg {
					return lessonCompletedMsg{lessonID: id, stars: stars, goNext: true}
				}
			}
			before := es.editor.Value()
			edCmd := es.editor.UpdateInPlace(msg)
			after := es.editor.Value()
			if after != before {
				if es.assembly.IsOpen() {
					es.assembly.MarkStale()
				}
				es.diagVersion++
				v := es.diagVersion
				diagCmd := tea.Tick(500*time.Millisecond, func(_ time.Time) tea.Msg {
					return diagTimerMsg{version: v}
				})
				return es, tea.Batch(edCmd, diagCmd)
			}
			return es, edCmd
		}

		if es.compositor.FocusID() == WidgetFileList {
			if msg.String() == KeySelect {
				if es.fileList != nil {
					if idx := es.fileList.Cursor(); idx != es.activeIdx {
						return es, backCmd(fileSelectedMsg{idx: idx})
					}
				}
			}
			return es, nil
		}

		if es.compositor.FocusID() == WidgetFlags && es.assembly.IsOpen() {
			before := es.flags.Value()
			flCmd := es.flags.UpdateInPlace(msg)
			if es.flags.Value() != before {
				es.assembly.MarkStale()
			}
			return es, flCmd
		}

	case compileResultMsg:
		es.compiling = false
		es.diag.SetCompiling(false)
		es.timer.Stop()
		duration := time.Since(es.runStarted)
		es.diag.SetDuration(duration)
		es.output.SetContent(es.formatResult(msg.result, msg.err))
		es.tests.Clear()
		if msg.result != nil && msg.result.Error != "" {
			es.console.SetContent(msg.result.Error)
		} else {
			es.console.Clear()
		}
		extra := []string{"compile"}
		if msg.result != nil {
			if msg.result.ExitCode == 139 {
				extra = append(extra, "segfault_king")
			}
			if strings.Contains(msg.result.Error, "timed out") {
				extra = append(extra, "into_the_loop")
			}
		}
		if msg.err == nil && msg.result != nil && msg.result.Error == "" && msg.result.ExitCode == 0 {
			es.lastHasWarnings = strings.Contains(msg.result.Output, "warning:")
			es.checkPassed(msg.result, es.lastHasWarnings)
		}
		events := buildAchievementEvents(es.passed, es.attempts, es.earnedStars, es.lastHasWarnings, es.lang, es.editor.Value(), extra...)
		return es, backCmd(achievementEventMsg{events: events})

	case testRunResultMsg:
		es.compiling = false
		es.diag.SetCompiling(false)
		es.timer.Stop()
		es.diag.SetDuration(time.Since(es.runStarted))
		if msg.err != nil {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.tool_error"), MoodWorried)
			es.output.SetContent(exErrorStyle.Render(es.loc.T("exercise.result.error", msg.err)))
			es.tests.Clear()
			return es, nil
		}
		if msg.compileResult.Error != "" {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.compile_error"), MoodWorried)
			es.output.SetContent(exErrorStyle.Render(msg.compileResult.Error))
			es.tests.Clear()
			return es, nil
		}
		es.lastHasWarnings = strings.Contains(msg.compileResult.Output, "warning:")
		es.checkAllTestsPassed(msg.testResults)
		extra := []string{"compile"}
		for _, r := range msg.testResults {
			if r.ExitCode == 139 {
				extra = append(extra, "segfault_king")
				break
			}
		}
		for _, r := range msg.testResults {
			if strings.Contains(r.Error, "timed out") {
				extra = append(extra, "into_the_loop")
				break
			}
		}
		events := buildAchievementEvents(es.passed, es.attempts, es.earnedStars, es.lastHasWarnings, es.lang, es.editor.Value(), extra...)
		return es, backCmd(achievementEventMsg{events: events})

	case debugResultMsg:
		es.compiling = false
		es.diag.SetCompiling(false)
		es.timer.Stop()
		es.diag.SetDuration(time.Since(es.runStarted))
		if msg.err != nil {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.tool_error"), MoodWorried)
			es.output.SetContent(exErrorStyle.Render(es.loc.T("exercise.result.error", msg.err)))
			return es, nil
		}
		r := msg.result
		// Strip ANSI codes that ASAN emits even to pipes; clean text is needed
		// for the output widget, the memory parser, and the hex viewer.
		cleanOutput := stripANSI(r.Output)
		cleanError := stripANSI(r.Error)
		if cleanError != "" {
			es.passed = false
			es.earnedStars = 0
			if isCrashText(cleanError) || r.ExitCode == 139 {
				es.mascot.Speak(es.loc.T("exercise.mochi.asan_crash_path"), MoodCrashed)
			} else {
				es.mascot.Speak(es.loc.T("exercise.mochi.asan_early"), MoodWorried)
			}
			es.output.SetContent(exErrorStyle.Render(cleanError))
			return es, nil
		}
		if r.ExitCode != 0 {
			es.passed = false
			es.earnedStars = 0
			if isCrashText(cleanOutput) {
				es.mascot.Speak(es.loc.T("exercise.mochi.asan_crash"), MoodCrashed)
			} else {
				es.mascot.Speak(es.loc.T("exercise.mochi.asan_memory"), MoodWorried)
			}
		}
		label := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAmber)).Render(es.loc.T("exercise.result.asan_label"))
		es.memory.SetAsanOutput(cleanOutput)
		es.memory.SetOpen(true)
		if cleanOutput != "" {
			es.hexViewer.SetHexDump(cleanOutput)
		}
		// Pre-render the combined memory widget + hex dump into the output widget
		// so that j/k in output mode scrolls both sections uniformly, with no
		// special-casing needed in renderOutputPanel.
		preRendered := label + "\n\n" + es.memory.View()
		if es.hexViewer.IsVisible() {
			preRendered += es.hexViewer.View(99999)
		}
		if r.ExitCode != 0 && cleanOutput == "" {
			preRendered += exErrorStyle.Render(es.loc.T("exercise.result.live_exit", r.ExitCode))
		}
		es.output.SetContent(preRendered)
		return es, backCmd(achievementEventMsg{events: []string{"asan"}})

	case gdbReadyMsg:
		es.compiling = false
		es.diag.SetCompiling(false)
		es.timer.Stop()
		es.diag.SetDuration(time.Since(es.runStarted))
		if msg.compileErr != nil {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.compiler_wall"), MoodWorried)
			es.output.SetContent(exErrorStyle.Render(msg.compileErr.Error))
			return es, nil
		}
		runtimeArgs := parseFlags(es.args.Value())
		if es.lang == nil || !es.lang.HasDebugger() {
			msg.build.Close()
			es.output.SetContent(exErrorStyle.Render("debugger not available"))
			return es, nil
		}
		gdbInitFile, err := os.CreateTemp("", "ztutor-gdbinit-*")
		if err != nil {
			msg.build.Close()
			es.output.SetContent(exErrorStyle.Render("create gdb init: " + err.Error()))
			return es, nil
		}
		gdbInitPath := gdbInitFile.Name()
		_, _ = gdbInitFile.WriteString("define shell\n")
		_, _ = gdbInitFile.WriteString("printf \"The shell command is disabled in the ztutor sandbox.\\n\"\n")
		_, _ = gdbInitFile.WriteString("end\n")
		gdbInitFile.Close()
		es.gdbInitPath = gdbInitPath
		gdbArgs := []string{"-q",
			"-ex", "set debuginfod enabled off",
			"-ex", "set confirm off",
			"-x", gdbInitPath}
		if len(runtimeArgs) > 0 {
			gdbArgs = append(gdbArgs, "--args", msg.build.BinaryPath)
			gdbArgs = append(gdbArgs, runtimeArgs...)
		} else {
			gdbArgs = append(gdbArgs, msg.build.BinaryPath)
		}
		writeFn, ch, kill, err := sandbox.RunDebugger(es.lang.DebuggerPath(), gdbArgs)
		if err != nil {
			msg.build.Close()
			es.output.SetContent(exErrorStyle.Render(err.Error()))
			return es, nil
		}
		es.running = true
		es.runMode = runModeGDB
		es.compositor.SetRunning(true)
		es.compositor.EnterFullscreen(WidgetOutput)
		es.interWrite = writeFn
		es.interKill = kill
		es.interCh = ch
		es.interBuild = msg.build
		es.liveOutput = ""
		es.programOutput = ""
		es.output.SetContent("")
		es.tests.Clear()
		es.editor.Blur()
		achCmd := backCmd(achievementEventMsg{events: []string{"gdb"}})
		return es, tea.Batch(waitForInteractive(ch), achCmd)

	case interactiveReadyMsg:
		es.compiling = false
		es.diag.SetCompiling(false)
		es.timer.Stop()
		es.diag.SetDuration(time.Since(es.runStarted))
		if msg.compileErr != nil {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.compiler_wall"), MoodWorried)
			es.output.SetContent(exErrorStyle.Render(msg.compileErr.Error))
			return es, nil
		}
		runtimeArgs := parseFlags(es.args.Value())
		var cmd string
		var fullArgs []string
		if es.lang != nil {
			cmd, fullArgs = es.lang.InteractiveCommand(msg.build.BinaryPath)
		} else {
			cmd = msg.build.BinaryPath
		}
		fullArgs = append(fullArgs, runtimeArgs...)
		writeFn, ch, kill, err := es.executor.RunInteractive(cmd, fullArgs)
		if err != nil {
			msg.build.Close()
			es.output.SetContent(exErrorStyle.Render(es.loc.T("exercise.result.run_error", err)))
			return es, nil
		}
		es.running = true
		es.runMode = runModeInteractive
		es.interWrite = writeFn
		es.interKill = kill
		es.interCh = ch
		es.interBuild = msg.build
		es.liveOutput = ""
		es.programOutput = ""
		es.output.SetContent("")
		es.tests.Clear()
		es.runInput.Reset()
		es.runInput.Focus()
		es.editor.Blur()
		achCmd := backCmd(achievementEventMsg{events: []string{"interactive"}})
		return es, tea.Batch(waitForInteractive(ch), achCmd)

	case programOutputMsg:
		text := msg.text
		if es.runMode == runModeGDB {
			text = stripANSI(text)
		}
		es.programOutput += text
		es.liveOutput += text
		es.output.SetContent(es.liveOutput)
		return es, waitForInteractive(es.interCh)

	case programDoneMsg:
		wasGDB := es.runMode == runModeGDB
		es.running = false
		es.runMode = runModeNone
		es.compositor.SetRunning(false)
		es.interWrite = nil
		es.interKill = nil
		if es.interBuild != nil {
			es.interBuild.Close()
			es.interBuild = nil
		}
		es.runInput.Blur()
		es.compositor.FocusEditor()
		if wasGDB {
			es.compositor.ExitFullscreen()
			os.Remove(es.gdbInitPath)
			es.gdbInitPath = ""
			es.mascot.Speak(es.loc.T("exercise.mochi.gdb_exit"), MoodHappy)
			return es, nil
		}
		if msg.code != 0 {
			es.passed = false
			es.earnedStars = 0
			if msg.code == 139 {
				es.mascot.Speak(es.loc.T("exercise.mochi.live_crash"), MoodCrashed)
				es.output.SetContent(es.liveOutput + exErrorStyle.Render("\n"+es.loc.T("exercise.result.segfault")))
				return es, backCmd(achievementEventMsg{events: []string{"segfault_king"}})
			}
			es.mascot.Speak(es.loc.T("exercise.mochi.live_error"), MoodWorried)
			es.output.SetContent(es.liveOutput + exErrorStyle.Render("\n"+es.loc.T("exercise.result.live_exit", msg.code)))
		} else {
			es.checkPassed(&sandbox.Result{Output: es.programOutput, Stdout: es.programOutput}, es.lastHasWarnings)
		}
		return es, nil

	case fileSelectedMsg:
		es.saveActiveContent()
		es.activeIdx = msg.idx
		if es.fileList != nil {
			es.fileList.SetActive(msg.idx)
		}
		af := es.activeFiles[msg.idx]
		if af.Lang != nil {
			es.lang = af.Lang
		}
		hlLang := af.File.Language
		if hlLang == "" && af.Lang != nil {
			hlLang = af.Lang.Name()
		}
		es.editor.SwitchFile(af.Content, hlLang)
		es.assembly.Close()
		es.assembly.SetLang(es.lang)
		es.diag.SetDiagnostics(nil)
		es.diagVersion++
		// Preserve fullscreen editor layout when switching files; reset otherwise
		// (e.g. if we were in asm-split, that assembly is now stale).
		if es.compositor.InFullscreen() && es.compositor.FullscreenID() == WidgetEditor {
			es.compositor.FocusEditorInPlace()
		} else {
			es.compositor.FocusEditor()
		}
		if es.lang != nil {
			return es, es.syntaxCheckCmd(parseFlags(es.flags.Value()), es.diagVersion)
		}
		return es, nil

	case diagTimerMsg:
		if msg.version == es.diagVersion && es.lang != nil {
			return es, es.syntaxCheckCmd(parseFlags(es.flags.Value()), msg.version)
		}
		return es, nil

	case diagResultMsg:
		if msg.version == es.diagVersion {
			es.diag.SetDiagnostics(msg.diags)
			es.updateEditorDiags()
		}
		return es, nil

	case asmResultMsg:
		if msg.err != nil {
			es.output.SetContent(exErrorStyle.Render(es.loc.T("exercise.result.asm_error", msg.err)))
			return es, nil
		}
		if strings.TrimSpace(msg.asm) == "" {
			es.output.SetContent(dim(es.loc.T("exercise.result.asm_empty")))
			return es, nil
		}
		lines := strings.Split(msg.asm, "\n")
		es.assembly.Open(lines, strings.TrimSpace(es.flags.Value()))
		if !es.compositor.InAsmMode() {
			// First compile or compile triggered while not in asm split:
			// auto-enter the side-by-side view.
			es.compositor.FocusAsm()
		}
		// Already in asm split (e.g. annotate re-compile): data updated,
		// view refreshes on next render — no focus change needed.
		return es, backCmd(achievementEventMsg{events: []string{"asm"}})

	case tea.WindowSizeMsg:
		es.HandleResize(msg)
		es.compositor.SetSize(msg.Width, msg.Height)
		es.mascot.SetWidth(msg.Width)
		es.diag.SetWidth(msg.Width)
		half := msg.Width / 2
		es.flags.SetSize(half, 0)
		es.args.SetSize(msg.Width-half, 0)
		es.stdin.SetSize(msg.Width, 0)
	}

	return es, nil
}

// currentFilesMap returns a snapshot of all exercise files, using the live
// editor content for the active file.
func (es *ExerciseScreen) currentFilesMap() map[string]string {
	if len(es.activeFiles) == 0 {
		name := "main.c"
		if es.lang != nil {
			name = es.lang.SourceFileName()
		}
		return map[string]string{name: es.editor.Value()}
	}
	m := make(map[string]string, len(es.activeFiles))
	for i, af := range es.activeFiles {
		content := af.Content
		if i == es.activeIdx {
			content = es.editor.Value()
		}
		m[af.File.Name] = content
	}
	return m
}

// saveActiveContent copies the editor's current value back to activeFiles so
// it's preserved when switching to a different file.
func (es *ExerciseScreen) saveActiveContent() {
	if es.activeIdx < len(es.activeFiles) {
		es.activeFiles[es.activeIdx].Content = es.editor.Value()
	}
}

func (es *ExerciseScreen) SetMascotFrame(frame int) {
	es.mascot.SetFrame(frame)
}

func arrowSeq(key tea.KeyType) string {
	switch key {
	case tea.KeyUp:
		return "\x1b[A"
	case tea.KeyDown:
		return "\x1b[B"
	case tea.KeyRight:
		return "\x1b[C"
	case tea.KeyLeft:
		return "\x1b[D"
	default:
		return ""
	}
}
