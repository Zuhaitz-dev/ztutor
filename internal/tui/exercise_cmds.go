package tui

import (
	"ztutor/internal/logutil"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

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

func waitForInteractive(ch <-chan sandbox.InteractiveEvent) tea.Cmd {
	return func() tea.Msg {
		ev := <-ch
		if ev.Done {
			return programDoneMsg{code: ev.Code}
		}
		return programOutputMsg{text: ev.Text}
	}
}
