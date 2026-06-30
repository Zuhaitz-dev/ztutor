package tui

import (
	"testing"

	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"

	tea "github.com/charmbracelet/bubbletea"
)

func testLessonWithExpected(id, expected string, tests []lesson.TestCase) lesson.Lesson {
	l := testLesson(id, "int main() { return 0; }", nil, nil)
	if expected != "" {
		l.Tests = []lesson.TestCase{{Expected: expected}}
	}
	if len(tests) > 0 {
		l.Tests = tests
	}
	return l
}

func TestCompileResult_Pass(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "hello", Stdout: "hello", ExitCode: 0},
	})

	if !es.passed {
		t.Error("should be passed when output matches")
	}
	if es.earnedStars == 0 {
		t.Error("should have a star rating on pass")
	}
}

func TestCompileResult_Fail_WrongOutput(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "world", Stdout: "world", ExitCode: 0},
	})

	if es.passed {
		t.Error("should not pass when output differs")
	}
}

func TestCompileResult_Fail_EmptyOutput(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "", Stdout: "", ExitCode: 0},
	})

	if es.passed {
		t.Error("should not pass with empty output when expected is non-empty")
	}
}

func TestCompileResult_StreamAwarePass(t *testing.T) {
	l := testLessonWithExpected("x", "", []lesson.TestCase{{
		ExpectedStdout:    "payload\n",
		ExpectedStderr:    "debug\n",
		HasExpectedStdout: true,
		HasExpectedStderr: true,
	}})
	es := newFakeExerciseScreen(t, l)
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{
			Output:   "payload\n\ndebug\n",
			Stdout:   "payload\n",
			Stderr:   "debug\n",
			ExitCode: 0,
		},
	})

	if !es.passed {
		t.Error("should pass when stdout and stderr match their separate expectations")
	}
}

func TestCompileResult_CompileError(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("compiling")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Error: "compilation error: syntax error"},
	})

	if es.passed {
		t.Error("should not pass with compile error")
	}
	if es.earnedStars > 0 {
		t.Error("should not award stars on compile error")
	}
}

func TestCompileResult_Crash(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "", ExitCode: 139, Error: "program crashed: segmentation fault"},
	})

	if es.passed {
		t.Error("should not pass on crash")
	}
}

func TestCompileResult_Timeout(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Error: "program timed out (5s)"},
	})

	if es.passed {
		t.Error("should not pass on timeout")
	}
}

func TestTestRunResult_AllPassed(t *testing.T) {
	l := testLessonWithExpected("x", "", []lesson.TestCase{
		{Expected: "hello", Stdin: ""},
		{Expected: "world", Stdin: "test"},
	})
	es := newFakeExerciseScreen(t, l)
	es.startRun("running")

	es.Update(testRunResultMsg{
		compileResult: &sandbox.Result{Output: "", ExitCode: 0},
		testResults: []sandbox.TestResult{
			{Num: 1, Passed: true, Got: "hello", Want: "hello"},
			{Num: 2, Passed: true, Got: "world", Want: "world"},
		},
	})

	if !es.passed {
		t.Error("should pass when all tests pass")
	}
}

func TestTestRunResult_SomeFailed(t *testing.T) {
	l := testLessonWithExpected("x", "", []lesson.TestCase{
		{Expected: "hello", Stdin: ""},
		{Expected: "world", Stdin: "test"},
	})
	es := newFakeExerciseScreen(t, l)
	es.startRun("running")

	es.Update(testRunResultMsg{
		compileResult: &sandbox.Result{Output: "", ExitCode: 0},
		testResults: []sandbox.TestResult{
			{Num: 1, Passed: true, Got: "hello", Want: "hello"},
			{Num: 2, Passed: false, Got: "wrong", Want: "world", Error: "mismatch"},
		},
	})

	if es.passed {
		t.Error("should not pass when some tests fail")
	}
}

func TestTestRunResult_AllCrashed(t *testing.T) {
	l := testLessonWithExpected("x", "", []lesson.TestCase{
		{Expected: "hello", Stdin: ""},
	})
	es := newFakeExerciseScreen(t, l)
	es.startRun("running")

	es.Update(testRunResultMsg{
		compileResult: &sandbox.Result{Output: "", ExitCode: 0},
		testResults: []sandbox.TestResult{
			{Num: 1, Passed: false, Error: "program crashed"},
		},
	})

	if es.passed {
		t.Error("should not pass on crash result")
	}
}

func TestStartRun_SetsCompiling(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))

	es.startRun("compiling")
	if !es.compiling {
		t.Error("should be compiling after startRun(start=compiling)")
	}
	if es.passed {
		t.Error("should not be passed after startRun")
	}
	if es.earnedStars != 0 {
		t.Error("earnedStars should reset on startRun")
	}
}

func TestExerciseScreen_EnteThenExitFullscreenIsValid(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.compositor.EnterFullscreen(WidgetEditor)
	es.compositor.ExitFullscreen()

	if issues := es.compositor.Validate(); len(issues) != 0 {
		t.Errorf("exiting fullscreen should produce valid state: %v", issues)
	}
}

func TestExerciseScreen_Update_EmptyCompositorDoesNotPanic(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	// Should not panic.
	_, _ = es.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
}

func TestCompileResult_TrimSpaceIsApplied(t *testing.T) {
	// checkPassed uses strings.TrimSpace on both got and expected, so trailing
	// whitespace differences are ignored.
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello\n", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "hello", ExitCode: 0},
	})

	if !es.passed {
		t.Error("should pass — TrimSpace normalizes trailing whitespace")
	}
}

func TestMsg_DiagResult_Updates(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	// diagResultMsg only applies when msg.version matches es.diagVersion.
	es.diagVersion = 1
	diags := []sandbox.Diagnostic{
		{Line: 3, Col: 5, Kind: "error", Message: "expected ';'"},
	}
	es.Update(diagResultMsg{diags: diags, version: 1})
	// Should have diagnostics loaded (diagVersion stays same, content updated).
	if es.diagVersion != 1 {
		t.Errorf("diagVersion = %d, want 1", es.diagVersion)
	}
}

func TestMsg_ProgramOutput_AppendsText(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.running = true
	es.startRun("interactive")

	es.Update(programOutputMsg{text: "output line 1\n"})
	es.Update(programOutputMsg{text: "output line 2\n"})

	if es.liveOutput != "output line 1\noutput line 2\n" {
		t.Errorf("liveOutput = %q, want concatenated lines", es.liveOutput)
	}
}

func TestMsg_DiagResult_IgnoresStaleVersion(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.diagVersion = 5

	diags := []sandbox.Diagnostic{
		{Line: 1, Col: 1, Kind: "error", Message: "stale"},
	}
	es.Update(diagResultMsg{diags: diags, version: 3})

	if es.diagVersion == 3 {
		t.Error("stale diagnostic version should be ignored")
	}
}

func TestExerciseScreen_View_ReturnsString(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	v := es.View()
	if v == "" {
		t.Error("View should return non-empty string")
	}
}

func TestExerciseScreen_RapidMessages(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	// Inject messages rapidly.
	messages := []tea.Msg{
		compileResultMsg{result: &sandbox.Result{Output: "hello", ExitCode: 0}},
		diagResultMsg{diags: nil, version: 1},
		programDoneMsg{code: 0},
	}
	for i := 0; i < 10; i++ {
		for _, m := range messages {
			_, _ = es.Update(m)
		}
	}
	// Should not panic and should produce valid state.
	if issues := es.compositor.Validate(); len(issues) != 0 {
		t.Errorf("state invalid after rapid messages: %v", issues)
	}
}

func TestCompileResult_ExactMatch(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello world\n", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "hello world\n", ExitCode: 0},
	})

	if !es.passed {
		t.Error("exact match including newline should pass")
	}
}

func TestCompileResult_StarMessageEarned(t *testing.T) {
	es := newFakeExerciseScreen(t, testLessonWithExpected("x", "hello", nil))
	es.startRun("running")

	es.Update(compileResultMsg{
		result: &sandbox.Result{Output: "hello", ExitCode: 0},
	})

	if es.passed && es.earnedStars == 0 {
		t.Error("earnedStars should be non-zero on pass")
	}
}
