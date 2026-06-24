package tui

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"ztutor/internal/i18n"
	"ztutor/internal/lesson"
	"ztutor/internal/sandbox"
)

func hasGCC() bool {
	_, err := exec.LookPath("gcc")
	return err == nil
}

func testLesson(id, exercise string, tests []lesson.TestCase, files []lesson.ExerciseFile) lesson.Lesson {
	return lesson.Lesson{
		ID:                 id,
		Title:              "Test Lesson",
		Exercise:           exercise,
		Tests:              tests,
		Files:              files,
		Language:           "c",
		SourceExtension:    ".c",
		SyntaxHighlighting: "c",
	}
}

func newTestExerciseScreen(t *testing.T, l lesson.Lesson) *ExerciseScreen {
	t.Helper()
	lang := sandbox.GetLanguage("c")
	exec := sandbox.DefaultExecutor()
	loc := i18n.New("en")
	return NewExerciseScreen(l, lang, exec, 80, 40, "vim", 0, 0, loc, true, true)
}

func TestIntegration_CompileAndRun(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-compile", `#include <stdio.h>
int main(void) {
    printf("42\n");
    return 0;
}`, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(void) {
    printf("42\n");
    return 0;
}`
	es.editor.SwitchFile(code, "c")
	es.compositor.FocusEditor()

	cmd := es.compileCmd("", nil, nil)
	msg := cmd()
	result, ok := msg.(compileResultMsg)
	if !ok {
		t.Fatalf("expected compileResultMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("compile error: %v", result.err)
	}
	if result.result.Error != "" {
		t.Fatalf("result error: %s", result.result.Error)
	}
	output := strings.TrimSpace(result.result.Output)
	if output != "42" {
		t.Errorf("output = %q, want %q", output, "42")
	}
}

func TestIntegration_CompileWithStdin(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-stdin", `#include <stdio.h>
int main(void) {
    char buf[64];
    fgets(buf, sizeof(buf), stdin);
    printf("echo: %s", buf);
    return 0;
}`, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(void) {
    char buf[64];
    fgets(buf, sizeof(buf), stdin);
    printf("echo: %s", buf);
    return 0;
}`
	es.editor.SwitchFile(code, "c")

	cmd := es.compileCmd("hello\n", nil, nil)
	msg := cmd()
	result := msg.(compileResultMsg)
	if result.err != nil {
		t.Fatalf("compile error: %v", result.err)
	}
	output := strings.TrimSpace(result.result.Output)
	if output != "echo: hello" {
		t.Errorf("output = %q, want %q", output, "echo: hello")
	}
}

func TestIntegration_SyntaxCheck(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-syntax", ``, nil, nil)
	es := newTestExerciseScreen(t, l)

	// Valid code: no diagnostics
	es.editor.SwitchFile(`#include <stdio.h>
int main(void) { return 0; }`, "c")
	cmd := es.syntaxCheckCmd(nil, 1)
	msg := cmd()
	diagMsg := msg.(diagResultMsg)
	errCount := 0
	for _, d := range diagMsg.diags {
		if d.Kind == "error" {
			errCount++
		}
	}
	if errCount > 0 {
		t.Errorf("expected 0 errors, got %d", errCount)
	}

	// Invalid code: should produce diagnostics
	es.editor.SwitchFile(`#include <stdio.h>
int main(void) {
    invalid syntax here
    return 0;
}`, "c")
	cmd2 := es.syntaxCheckCmd(nil, 2)
	msg2 := cmd2()
	diagMsg2 := msg2.(diagResultMsg)
	if len(diagMsg2.diags) == 0 {
		t.Fatal("expected at least one diagnostic")
	}
}

func TestIntegration_GenerateAssembly(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-asm", ``, nil, nil)
	es := newTestExerciseScreen(t, l)

	es.editor.SwitchFile(`int add(int a, int b) { return a + b; }`, "c")
	cmd := es.asmCmd(nil)
	msg := cmd()
	asmMsg := msg.(asmResultMsg)
	if asmMsg.err != nil {
		t.Fatalf("asm error: %v", asmMsg.err)
	}
	if asmMsg.asm == "" {
		t.Fatal("empty assembly output")
	}
	if !strings.Contains(asmMsg.asm, "add") {
		t.Error("assembly should contain function name 'add'")
	}
}

func TestIntegration_MultiTest(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-multi", `#include <stdio.h>
int main(void) {
    int n;
    scanf("%d", &n);
    if (n > 0) printf("positive\n");
    else if (n < 0) printf("negative\n");
    else printf("zero\n");
    return 0;
}`, []lesson.TestCase{
		{Stdin: "5\n", Expected: "positive"},
		{Stdin: "-3\n", Expected: "negative"},
		{Stdin: "0\n", Expected: "zero"},
	}, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(void) {
    int n;
    scanf("%d", &n);
    if (n > 0) printf("positive\n");
    else if (n < 0) printf("negative\n");
    else printf("zero\n");
    return 0;
}`
	es.editor.SwitchFile(code, "c")

	tests := []sandbox.TestInput{
		{Stdin: "5\n", Expected: "positive"},
		{Stdin: "-3\n", Expected: "negative"},
		{Stdin: "0\n", Expected: "zero"},
	}

	cmd := es.runAllTestsCmd(nil, tests)
	msg := cmd()
	result := msg.(testRunResultMsg)
	if result.err != nil {
		t.Fatalf("run error: %v", result.err)
	}
	if result.compileResult.Error != "" {
		t.Fatalf("compile error: %s", result.compileResult.Error)
	}
	if len(result.testResults) != 3 {
		t.Fatalf("expected 3 test results, got %d", len(result.testResults))
	}
	for i, r := range result.testResults {
		if !r.Passed {
			t.Errorf("test %d failed: got=%q want=%q", i+1, r.Got, r.Want)
		}
	}
}

func TestIntegration_StarCalculation(t *testing.T) {
	// Pure logic, no gcc needed
	tests := []struct {
		attempts, hints int
		hasWarnings     bool
		want            int
	}{
		{1, 0, false, 3},
		{2, 0, false, 3},
		{3, 0, false, 2},
		{1, 0, true, 2},
		{1, 1, false, 2},
		{1, 1, true, 2},
		{3, 2, false, 1},
		{3, 0, true, 1},
		{10, 0, false, 2},
	}
	for _, tc := range tests {
		got := calculateStars(tc.attempts, tc.hints, tc.hasWarnings)
		if got != tc.want {
			t.Errorf("calculateStars(%d, %d, %v) = %d, want %d",
				tc.attempts, tc.hints, tc.hasWarnings, got, tc.want)
		}
	}
}

func TestIntegration_CompileError(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-err", ``, nil, nil)
	es := newTestExerciseScreen(t, l)

	cmd := es.compileCmd("", nil, nil)
	msg := cmd()
	result := msg.(compileResultMsg)
	if result.err != nil {
		t.Fatalf("unexpected error: %v", result.err)
	}
	if result.result.Error == "" {
		t.Fatal("expected compilation error for empty code")
	}
}

func TestIntegration_InteractiveCompile(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-inter", ``, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(void) {
    printf("interactive\n");
    return 0;
}`
	es.editor.SwitchFile(code, "c")

	cmd := es.interactiveCompileCmd(nil)
	msg := cmd()
	interMsg := msg.(interactiveReadyMsg)
	if interMsg.compileErr != nil {
		t.Fatalf("compile error: %s", interMsg.compileErr.Error)
	}
	if interMsg.build == nil {
		t.Fatal("build is nil")
	}
	defer interMsg.build.Close()

	write, ch, kill, err := sandbox.DefaultExecutor().RunInteractive(interMsg.build.BinaryPath, nil)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	defer kill()

	select {
	case ev := <-ch:
		if !ev.Done {
			t.Logf("output: %q", ev.Text)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for output")
	}
	_ = write
}
