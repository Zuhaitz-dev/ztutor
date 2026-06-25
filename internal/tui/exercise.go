package tui

import (
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
	interWrite    func([]byte) error
	interKill     func()
	interCh       <-chan sandbox.InteractiveEvent
	interBuild    *sandbox.DebugBuild
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

func (es *ExerciseScreen) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if es.running {
			switch msg.String() {
			case KeyQuit:
				if es.interKill != nil {
					es.interKill()
				}
			case KeyBackEditor:
				if es.interKill != nil {
					es.interKill()
				}
				return es, backCmd(NavigateToMenu{})
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

		switch msg.String() {
		case KeyBackEditor:
			return es, backCmd(NavigateToMenu{})

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
			if !es.hint.Available() {
				return es, nil
			}
			es.hint.Next()
			es.mascot.Speak(es.loc.T("exercise.mochi.hint", es.hint.CurrentIndex()+1), MoodCurious)
			return es, nil

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
			if es.trivia.Available() {
				es.trivia.Next()
				es.mascot.Speak(es.trivia.Current(), MoodCurious)
			}
			return es, nil

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
			es.checkPassed(msg.result.Output, es.lastHasWarnings)
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
		msg.build.RuntimeArgs = parseFlags(es.args.Value())
		launchCmd := backCmd(launchGDBMsg{build: msg.build, lesson: es.lesson})
		achCmd := backCmd(achievementEventMsg{events: []string{"gdb"}})
		return es, tea.Batch(launchCmd, achCmd)

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
		es.programOutput += msg.text
		es.liveOutput += msg.text
		es.output.SetContent(es.liveOutput)
		return es, waitForInteractive(es.interCh)

	case programDoneMsg:
		es.running = false
		es.interWrite = nil
		es.interKill = nil
		if es.interBuild != nil {
			es.interBuild.Close()
			es.interBuild = nil
		}
		es.runInput.Blur()
		es.compositor.FocusEditor()
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
			es.checkPassed(es.programOutput, es.lastHasWarnings)
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

func isCrashText(text string) bool {
	lower := strings.ToLower(text)
	return strings.Contains(lower, "segmentation fault") ||
		strings.Contains(lower, "sigsegv") ||
		strings.Contains(lower, "program crashed") ||
		strings.Contains(lower, "deadlysignal") ||
		strings.Contains(lower, "exited with code 139")
}

// codeEasterEggLine checks source for known patterns and returns a special companion
// line to display before running, or "" if none match.
func codeEasterEggLine(lang sandbox.Language, code string, loc *i18n.Locale) string {
	switch {
	case lang != nil && lang.Name() == "c" && strings.Contains(code, "#include <beer.h>"):
		return loc.T("exercise.egg.beer")
	case strings.Contains(code, "void main("):
		return loc.T("exercise.egg.void_main")
	case strings.Contains(code, "goto "):
		return loc.T("exercise.egg.goto")
	}
	return ""
}

func (es *ExerciseScreen) mascotMood() MascotMood {
	// Transitional states always override — they are temporary and obvious.
	if es.compiling || es.running {
		return MoodThinking
	}
	// Message handlers pin a specific mood via Speak(); use it when set.
	if pin := es.mascot.PinnedMood(); pin != "" {
		return pin
	}
	// Fallback: derive from exercise state.
	switch {
	case es.passed:
		return MoodHappy
	case es.hint.IsVisible():
		return MoodCurious
	case es.compositor.InAsmMode() || es.compositor.InOutputMode() || es.compositor.FocusID() == WidgetFileList:
		return MoodFocused
	default:
		return MoodIdle
	}
}

func (es *ExerciseScreen) startRun(line string) {
	es.compiling = true
	es.passed = false
	es.earnedStars = 0
	es.hint.Hide()
	es.output.SetContent("")
	es.tests.Clear()
	es.memory.Close()
	es.hexViewer.Clear()
	es.diag.SetCompiling(true)
	es.mascot.ClearPin()
	es.mascot.SetLine(line)
	es.runStarted = time.Now()
	es.timer.Start()
}

func calculateStars(attempts, hintsUsed int, hasWarnings bool) int {
	fewAttempts := attempts <= 2
	var stars int
	switch {
	case fewAttempts && !hasWarnings:
		stars = 3
	case fewAttempts || !hasWarnings:
		stars = 2
	default:
		stars = 1
	}
	// Each hint used reduces the star ceiling.
	switch {
	case hintsUsed >= 2 && stars > 1:
		stars = 1
	case hintsUsed == 1 && stars > 2:
		stars = 2
	}
	return stars
}

func (es *ExerciseScreen) successMascotLine(stars, attempts int, hasWarnings bool) string {
	switch {
	case stars == 3:
		return es.loc.T("exercise.mochi.success_perfect")
	case hasWarnings:
		return es.loc.T("exercise.mochi.success_warnings")
	case attempts > 2:
		return es.loc.T("exercise.mochi.success_attempts")
	default:
		return es.loc.T("exercise.mochi.success")
	}
}

func (es *ExerciseScreen) wrongOutputMascotLine(got, want string) string {
	if strings.TrimSpace(got) == "" {
		return es.loc.T("exercise.mochi.wrong_empty")
	}
	if strings.Contains(got, "\n") != strings.Contains(want, "\n") {
		return es.loc.T("exercise.mochi.wrong_newlines")
	}
	return es.loc.T("exercise.mochi.wrong_output")
}

func (es *ExerciseScreen) testFailureMascotLine(r sandbox.TestResult) string {
	if isCrashText(r.Error) || r.ExitCode == 139 {
		return es.loc.T("exercise.mochi.test_crash")
	}
	if r.Error != "" {
		return es.loc.T("exercise.mochi.test_error")
	}
	return es.wrongOutputMascotLine(r.Got, r.Want)
}

// diffOutput renders a line-by-line diff between got and want.
func diffOutput(got, want string) string {
	gotLines := strings.Split(strings.TrimSpace(got), "\n")
	wantLines := strings.Split(strings.TrimSpace(want), "\n")
	addSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	delSt := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	maxLen := len(gotLines)
	if len(wantLines) > maxLen {
		maxLen = len(wantLines)
	}
	var b strings.Builder
	for i := 0; i < maxLen; i++ {
		wLine := ""
		if i < len(wantLines) {
			wLine = wantLines[i]
		}
		gLine := ""
		if i < len(gotLines) {
			gLine = gotLines[i]
		}
		if wLine == gLine {
			b.WriteString(dim("  " + wLine))
		} else {
			if i < len(wantLines) {
				b.WriteString(addSt.Render("+ " + wLine))
				b.WriteString("\n")
			}
			if i < len(gotLines) {
				b.WriteString(delSt.Render("- " + gLine))
			}
		}
		if i < maxLen-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (es *ExerciseScreen) checkPassed(output string, hasWarnings bool) {
	if len(es.lesson.Tests) == 0 {
		es.passed = true
		es.earnedStars = calculateStars(es.attempts, es.hint.HintsUsed(), hasWarnings)
		es.mascot.Speak(es.successMascotLine(es.earnedStars, es.attempts, hasWarnings), MoodHappy)
		es.output.SetContent(exSuccessStyle.Render(es.loc.T("exercise.result.compiled")) + "\n" + es.starMessage() + "\n\n" + output)
		return
	}
	got := strings.TrimSpace(output)
	want := strings.TrimSpace(es.lesson.Tests[0].Expected)
	if got == want {
		es.passed = true
		es.earnedStars = calculateStars(es.attempts, es.hint.HintsUsed(), hasWarnings)
		es.mascot.Speak(es.successMascotLine(es.earnedStars, es.attempts, hasWarnings), MoodHappy)
		es.output.SetContent(exSuccessStyle.Render(es.loc.T("exercise.result.correct")) + "\n" + es.starMessage() + "\n\n" + output)
	} else {
		es.passed = false
		es.earnedStars = 0
		es.mascot.Speak(es.wrongOutputMascotLine(output, es.lesson.Tests[0].Expected), MoodWorried)
		diff := diffOutput(output, es.lesson.Tests[0].Expected)
		es.output.SetContent(exErrorStyle.Render(es.loc.T("exercise.result.mismatch")) + "\n" + dim(es.loc.T("exercise.result.diff_hint")) + "\n\n" + diff)
	}
}

// scoreboard renders a compact per-test pass/fail row: [✓][✓][✗]
func scoreboard(results []sandbox.TestResult) string {
	passStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess))
	failStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorError))
	var b strings.Builder
	for _, r := range results {
		if r.Passed {
			b.WriteString(passStyle.Render("[✓]"))
		} else {
			b.WriteString(failStyle.Render("[✗]"))
		}
	}
	return b.String()
}

func (es *ExerciseScreen) checkAllTestsPassed(results []sandbox.TestResult) {
	passed, total := 0, len(results)
	for _, r := range results {
		if r.Passed {
			passed++
		}
	}
	es.tests.SetResults(results)
	es.progress.SetResult(total, passed)
	board := scoreboard(results)
	if passed == total {
		es.passed = true
		es.earnedStars = calculateStars(es.attempts, es.hint.HintsUsed(), es.lastHasWarnings)
		es.mascot.Speak(es.successMascotLine(es.earnedStars, es.attempts, es.lastHasWarnings), MoodHappy)
		header := exSuccessStyle.Render(es.loc.T("exercise.result.all_passed", total))
		es.output.SetContent(header + "  " + board + "\n" + es.starMessage())
		es.tests.Clear() // pass case: summary is enough; no per-test diff needed
	} else {
		es.passed = false
		es.earnedStars = 0
		var firstFail *sandbox.TestResult
		for i := range results {
			if !results[i].Passed {
				firstFail = &results[i]
				break
			}
		}
		if firstFail != nil {
			es.mascot.Speak(es.testFailureMascotLine(*firstFail), MoodWorried)
		}
		// Header goes to output; detailed diffs go to tests widget (shown in panel).
		header := exErrorStyle.Render(es.loc.T("exercise.result.tests_failed", passed, total)) + "  " + board
		es.output.SetContent(header + "\n\n" + es.tests.View())
		es.tests.Clear() // content is now baked into output; widget is done
	}
}

func (es *ExerciseScreen) starMessage() string {
	stars := es.earnedStars
	filled := strings.Repeat("★", stars)
	empty := strings.Repeat("☆", 3-stars)
	starStr := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Bold(true).Render(filled + empty)

	T := es.loc.T
	var reason string
	switch stars {
	case 3:
		reason = dim(T("exercise.stars.perfect", es.attempts))
	case 2:
		if es.attempts <= 2 {
			reason = dim(T("exercise.stars.warnings"))
		} else {
			reason = dim(T("exercise.stars.attempts", es.attempts))
		}
	default:
		reason = dim(T("exercise.stars.default"))
	}

	suffix := ""
	if es.previousStars > 0 && stars > es.previousStars {
		suffix = "  " + lipgloss.NewStyle().Foreground(lipgloss.Color(ColorSuccess)).Render(T("exercise.stars.new_best"))
	} else if es.previousStars > 0 && stars <= es.previousStars {
		suffix = "  " + dim(T("exercise.stars.prev_best", strings.Repeat("★", es.previousStars)+strings.Repeat("☆", 3-es.previousStars)))
	}

	hintNote := ""
	if es.hint.HintsUsed() > 0 {
		hintNote = "  " + dim(es.loc.T("exercise.mochi.hint_used", es.hint.HintsUsed()))
	}

	return starStr + "  " + reason + hintNote + suffix
}

func (es *ExerciseScreen) formatResult(r *sandbox.Result, err error) string {
	if err != nil {
		return exErrorStyle.Render(es.loc.T("exercise.result.error", err))
	}
	if r.Error != "" {
		if strings.Contains(strings.ToLower(r.Error), "compilation error") {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.compiler_wall_simple"), MoodWorried)
		} else if strings.Contains(r.Error, "timed out") {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.timeout"), MoodWorried)
		} else if isCrashText(r.Error) {
			es.passed = false
			es.earnedStars = 0
			es.mascot.Speak(es.loc.T("exercise.mochi.crash"), MoodCrashed)
		} else {
			es.mascot.Speak(es.loc.T("exercise.mochi.runtime_error"), MoodWorried)
		}
		return exErrorStyle.Render(r.Error)
	}
	out := r.Output
	if r.ExitCode != 0 {
		es.passed = false
		es.earnedStars = 0
		if r.ExitCode == 139 || isCrashText(out) {
			es.mascot.Speak(es.loc.T("exercise.mochi.segfault"), MoodCrashed)
		} else {
			es.mascot.Speak(es.loc.T("exercise.mochi.exit_code", r.ExitCode), MoodWorried)
		}
		if out != "" {
			out += "\n\n"
		}
		out += exErrorStyle.Render(es.loc.T("exercise.result.exit_nonzero", r.ExitCode))
	}
	return out
}

func (es *ExerciseScreen) View() string {
	if es.kbOverlay.IsVisible() {
		content := es.kbOverlay.View(es.loc, es.Width, es.Height)
		c := NewCanvas(es.Width, es.Height)
		c.DrawAt(0, content, lipgloss.Height(content))
		return c.String()
	}

	if es.compositor.InFullscreen() {
		return es.renderFullscreen()
	}

	T := es.loc.T
	l := NewTerminalLayout(es.Width, es.Height)

	l.AddFixed("header", nil, func(w int) string {
		langName := ""
		if es.lang != nil {
			langName = es.lang.Name()
		}
		title := exHeaderStyle.Render(es.lesson.Title)
		if langName != "" {
			title += "  " + dim("["+langName+"]")
		}
		return title + "\n" + dim(T("exercise.subtitle")) + "\n"
	})

	// Both sides of the split adjust together so each Ctrl+Up/Down step moves
	// the divider by one unit out of 10, giving ~3-5% change per keypress.
	// ASAN active overrides to a fixed 3:7 split so memory+hex have room.
	editorWeight := 10 - es.outputSplit
	outputWeight := es.outputSplit
	if es.memory.IsOpen() && es.hexViewer.IsVisible() {
		editorWeight, outputWeight = 3, 7
	}

	l.AddFlex("editor", editorWeight, nil, func(w, h int) string {
		return es.renderEditorArea(w, h)
	})

	l.AddFixed("inputs", nil, func(w int) string {
		if es.running {
			return es.renderRunningInput()
		}
		return es.renderInputsSection(w)
	})

	l.AddFlex("output", outputWeight, nil, func(w, h int) string {
		es.compositor.SetOutputH(h)
		return es.renderOutputPanel(w, h)
	})

	l.AddFixed("extras", func() bool {
		return es.timer.IsVisible() || es.progress.IsVisible()
	}, func(w int) string {
		var parts []string
		if es.timer.IsVisible() {
			parts = append(parts, es.timer.View())
		}
		if es.progress.IsVisible() {
			parts = append(parts, es.progress.View())
		}
		return strings.Join(parts, "\n")
	})

	l.AddFixed("streak", func() bool {
		return es.streak.IsVisible()
	}, func(w int) string {
		return es.streak.View()
	})

	l.AddFixed("console", func() bool {
		return es.console.IsVisible()
	}, func(w int) string {
		return es.console.View()
	})

	l.AddFixed("structinsp", func() bool {
		return es.structInsp.IsVisible()
	}, func(w int) string {
		return es.structInsp.View()
	})

	l.AddFixed("diag", nil, func(w int) string {
		return es.diag.View()
	})

	l.AddFixed("mascot", func() bool {
		return !es.mascot.IsHidden()
	}, func(w int) string {
		es.mascot.SetMood(es.mascotMood())
		return es.mascot.View()
	})

	l.AddFixed("helpbar", nil, func(w int) string {
		return es.compositor.HelpBar(es.passed, es.running, es.fileList != nil && es.fileList.Available(), es.loc)
	})

	return l.Render()
}

func (es *ExerciseScreen) renderEditorArea(w, h int) string {
	if h < 1 {
		h = 1
	}

	// Reserve one row for the VIM mode status line when applicable.
	modeLine := es.vimStatusLine()
	modeH := 0
	if modeLine != "" && h > 1 && !es.compositor.InFullscreen() {
		modeH = 1
	}
	contentH := h - modeH

	es.compositor.editorH = contentH
	if es.compositor.InAsmMode() {
		es.assembly.SetCurrentFlags(es.flags.Value())
		return es.assembly.RenderSideBySide(es.editor, w, contentH, true)
	}

	edW := w - editormod.LineNumWidth - 2
	if es.fileList != nil && es.fileList.Available() {
		edW -= fileListWidth + 1
	}
	if edW < 10 {
		edW = 10
	}
	es.editor.SetSize(edW, contentH)

	var result string

	if es.fileList != nil && es.fileList.Available() {
		es.fileList.SetSize(fileListWidth, contentH)
		listLines := strings.Split(es.fileList.View(), "\n")
		editorLines := strings.Split(es.editor.View(), "\n")

		divSt := fileListDividerStyle
		if es.compositor.FocusID() == WidgetFileList {
			divSt = fileListDividerFocStyle
		}
		div := divSt.Render("│")

		isRTL := es.loc.IsRTL()
		var out strings.Builder
		for i := 0; i < contentH; i++ {
			var left, right string
			if i < len(listLines) {
				left = listLines[i]
			}
			if i < len(editorLines) {
				right = editorLines[i]
			}
			if isRTL {
				left, right = right, left
			}
			pad := fileListWidth - lipgloss.Width(left)
			if pad < 0 {
				pad = 0
			}
			out.WriteString(left)
			out.WriteString(strings.Repeat(" ", pad))
			out.WriteString(div)
			out.WriteString(right)
			if i < contentH-1 {
				out.WriteString("\n")
			}
		}
		result = out.String()
	} else {
		result = es.editor.View()
	}

	if modeLine != "" {
		result += "\n" + modeLine
	}
	return result
}

// vimStatusLine returns a styled VIM mode indicator when applicable:
//   - keymap is "vim"
//   - editor is the focused widget
//   - editor reports a non-empty mode
//
// Returns empty string otherwise.
func (es *ExerciseScreen) vimStatusLine() string {
	if es.keymap != "vim" || es.compositor.FocusID() != WidgetEditor {
		return ""
	}
	mode := es.editor.Mode()
	if mode == "" {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).
		Foreground(lipgloss.Color(ColorAccent)).
		Render("-- " + mode + " --")
}

func (es *ExerciseScreen) renderInputsSection(w int) string {
	half := w / 2
	es.flags.SetSize(half, 0)
	es.args.SetSize(w-half, 0)
	flagsView := es.flags.View()
	if pad := half - lipgloss.Width(flagsView); pad > 0 {
		flagsView += strings.Repeat(" ", pad)
	}
	es.stdin.SetSize(w, 0)
	return flagsView + es.args.View() + "\n" + es.stdin.View()
}

func (es *ExerciseScreen) renderRunningInput() string {
	T := es.loc.T
	runStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(ColorAmber))
	return runStyle.Render(T("exercise.running_label")) + "  " + dim(T("exercise.running_hint")) + "\n" +
		flagsLabelFocusedStyle.Render(T("exercise.label.input")) +
		es.runInput.View() + flagsLabelStyle.Render("]")
}

func (es *ExerciseScreen) renderOutputPanel(w, h int) string {
	borderW := w - 4
	outBorderColor := lipgloss.Color("240")
	if es.compositor.InOutputMode() {
		outBorderColor = lipgloss.Color(ColorAccent)
	}
	bordered := func(content string) string {
		return exOutputStyle.BorderForeground(outBorderColor).Width(borderW).Height(h).Render(content)
	}

	if es.reference.IsVisible() {
		return bordered(es.reference.View())
	}

	T := es.loc.T
	var outputView string
	switch {
	case es.hint.IsVisible():
		outputView = es.hint.View()
	case es.compiling:
		outputView = dim(T("exercise.status.compiling"))
	case es.running && es.output.Content() == "":
		outputView = dim(T("exercise.status.waiting"))
	default:
		outputView = es.output.ViewScrolled(h)
	}
	if outputView == "" && es.previousStars > 0 {
		filled := strings.Repeat("★", es.previousStars)
		empty := strings.Repeat("☆", 3-es.previousStars)
		starDisp := lipgloss.NewStyle().Foreground(lipgloss.Color(ColorGold)).Render(filled + empty)
		if es.previousStars == 3 {
			outputView = starDisp + "  " + dim(T("exercise.status.perfect_prev"))
		} else {
			outputView = dim(T("exercise.status.prev_best_prefix")) + starDisp + dim(T("exercise.status.prev_best_suffix"))
		}
	}
	if outputView == "" {
		outputView = dim(T("exercise.status.empty_output"))
	}

	return bordered(outputView)
}

func (es *ExerciseScreen) renderFullscreen() string {
	l := NewTerminalLayout(es.Width, es.Height)

	l.AddFlex("widget", 1, nil, func(w, h int) string {
		switch es.compositor.FullscreenID() {
		case WidgetEditor:
			return es.renderEditorArea(w, h)
		case WidgetAssembly:
			return es.renderAssemblyFullscreen(w, h)
		case WidgetOutput:
			es.compositor.SetOutputH(h)
			return es.renderOutputPanel(w, h)
		}
		return ""
	})

	if es.compositor.FullscreenID() == WidgetEditor {
		l.AddFixed("vim", func() bool {
			return es.vimStatusLine() != ""
		}, func(w int) string {
			return es.vimStatusLine()
		})
	}

	l.AddFixed("helpbar", nil, func(w int) string {
		return helpBar(es.loc.T("exercise.help.fullscreen_exit"), es.loc.T("exercise.help.fullscreen_back"))
	})

	return l.Render()
}

func (es *ExerciseScreen) renderAssemblyFullscreen(w, h int) string {
	if h < 1 {
		h = 1
	}
	panelW := w - 6
	if panelW < 20 {
		panelW = 20
	}
	contentH := h - 2
	if contentH < 1 {
		contentH = 1
	}
	lines := es.assembly.RenderLines(contentH, panelW, es.flags.Value())

	content := strings.Join(lines, "\n")
	borderStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(ColorAccent)).
		Padding(0, 1).
		Width(w - 4).
		Height(h)

	return borderStyle.Render(content)
}

func (es *ExerciseScreen) compileCmd(stdin string, extraFlags, runtimeArgs []string) tea.Cmd {
	files := es.currentFilesMap()
	lang := es.lang
	buildCmd := es.lesson.BuildCmd
	executor := es.executor
	return func() tea.Msg {
		result, err := executor.Run(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
		return compileResultMsg{result: result, err: err}
	}
}

func (es *ExerciseScreen) runAllTestsCmd(extraFlags []string, tests []sandbox.TestInput) tea.Cmd {
	files := es.currentFilesMap()
	lang := es.lang
	buildCmd := es.lesson.BuildCmd
	executor := es.executor
	return func() tea.Msg {
		compileRes, testResults, err := executor.RunAllTests(lang, files, buildCmd, extraFlags, tests)
		return testRunResultMsg{compileResult: compileRes, testResults: testResults, err: err}
	}
}

func (es *ExerciseScreen) debugCmd(stdin string, extraFlags, runtimeArgs []string) tea.Cmd {
	files := es.currentFilesMap()
	lang := es.lang
	buildCmd := es.lesson.BuildCmd
	executor := es.executor
	return func() tea.Msg {
		result, err := executor.RunWithASAN(lang, files, buildCmd, stdin, extraFlags, runtimeArgs)
		return debugResultMsg{result: result, err: err}
	}
}

func (es *ExerciseScreen) gdbCompileCmd(extraFlags []string) tea.Cmd {
	files := es.currentFilesMap()
	lang := es.lang
	buildCmd := es.lesson.BuildCmd
	executor := es.executor
	return func() tea.Msg {
		build, compileErr := executor.CompileDebug(lang, files, buildCmd, extraFlags)
		return gdbReadyMsg{build: build, compileErr: compileErr}
	}
}

func (es *ExerciseScreen) asmCmd(extraFlags []string) tea.Cmd {
	lang := es.lang
	executor := es.executor
	files := es.currentFilesMap()
	return func() tea.Msg {
		asm, err := executor.GenerateAssembly(lang, files, "", extraFlags)
		return asmResultMsg{asm: asm, err: err}
	}
}

func (es *ExerciseScreen) interactiveCompileCmd(extraFlags []string) tea.Cmd {
	files := es.currentFilesMap()
	lang := es.lang
	buildCmd := es.lesson.BuildCmd
	executor := es.executor
	return func() tea.Msg {
		build, compileErr := executor.CompileDebug(lang, files, buildCmd, extraFlags)
		return interactiveReadyMsg{build: build, compileErr: compileErr}
	}
}

func (es *ExerciseScreen) syntaxCheckCmd(extraFlags []string, version int) tea.Cmd {
	lang := es.lang
	executor := es.executor
	files := es.currentFilesMap()
	return func() tea.Msg {
		diags, err := executor.SyntaxCheck(lang, files, "", extraFlags)
		if err != nil {
			logutil.Error("exercise: syntax check: %v", err)
		}
		return diagResultMsg{diags: diags, version: version}
	}
}

// updateEditorDiags pushes the current diagnostics into the editor's gutter map.
func (es *ExerciseScreen) updateEditorDiags() {
	diags := es.diag.Get()
	m := make(map[int]string, len(diags))
	for _, d := range diags {
		// Prefer "error" over "warning"/"note" when multiple messages share a line.
		if existing, ok := m[d.Line]; !ok || existing != "error" {
			m[d.Line] = d.Kind
		}
	}
	es.editor.SetDiagnostics(m)
}

func buildAchievementEvents(passed bool, attempts, stars int, lastHasWarnings bool, lang sandbox.Language, code string, extra ...string) []string {
	events := make([]string, 0, 10)
	events = append(events, extra...)
	if passed {
		events = append(events, "pass")
		if attempts == 1 {
			events = append(events, "pass_1attempt")
		}
		if attempts >= 5 {
			events = append(events, "pass_5attempts")
		}
		if stars == 3 {
			events = append(events, "pass_3star")
		}
		if !lastHasWarnings {
			events = append(events, "pass_nowarnings")
		}
	}
	if lang != nil && lang.Name() == "c" && strings.Contains(code, "#include <beer.h>") {
		events = append(events, "beer")
	}
	return events
}

func waitForInteractive(ch <-chan sandbox.InteractiveEvent) tea.Cmd {
	return func() tea.Msg {
		ev := <-ch
		if ev.Done {
			return programDoneMsg{code: ev.Code}
		}
		return programOutputMsg{text: ev.Text}
	}
}
