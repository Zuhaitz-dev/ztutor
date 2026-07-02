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

func TestIntegration_StreamAwareOutput(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-stream", `#include <stdio.h>
int main(void) {
    fprintf(stdout, "payload\n");
    fprintf(stderr, "debug: init\n");
    return 0;
}`, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(void) {
    fprintf(stdout, "payload\n");
    fprintf(stderr, "debug: init\n");
    return 0;
}`
	es.editor.SwitchFile(code, "c")

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
	if !strings.Contains(result.result.Stdout, "payload") {
		t.Errorf("Stdout = %q, want to contain 'payload'", result.result.Stdout)
	}
	if !strings.Contains(result.result.Stderr, "debug: init") {
		t.Errorf("Stderr = %q, want to contain 'debug: init'", result.result.Stderr)
	}
}

func TestIntegration_CompileWithFlags(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-flags", `#ifdef TEST_MODE
#include <stdio.h>
int main(void) { printf("test-mode\n"); return 0; }
#else
#include <stdio.h>
int main(void) { printf("production\n"); return 0; }
#endif`, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#ifdef TEST_MODE
#include <stdio.h>
int main(void) { printf("test-mode\n"); return 0; }
#else
#include <stdio.h>
int main(void) { printf("production\n"); return 0; }
#endif`
	es.editor.SwitchFile(code, "c")

	cmd := es.compileCmd("", []string{"-DTEST_MODE"}, nil)
	msg := cmd()
	result := msg.(compileResultMsg)
	if result.err != nil {
		t.Fatalf("compile error: %v", result.err)
	}
	if result.result.Error != "" {
		t.Fatalf("result error: %s", result.result.Error)
	}
	if strings.TrimSpace(result.result.Output) != "test-mode" {
		t.Errorf("output = %q, want %q", result.result.Output, "test-mode")
	}
}

func TestIntegration_CompileWithArgs(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-args", `#include <stdio.h>
int main(int argc, char *argv[]) {
    for (int i = 1; i < argc; i++) printf("[%d] %s\n", i, argv[i]);
    return 0;
}`, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(int argc, char *argv[]) {
    for (int i = 1; i < argc; i++) printf("[%d] %s\n", i, argv[i]);
    return 0;
}`
	es.editor.SwitchFile(code, "c")

	cmd := es.compileCmd("", nil, []string{"--verbose", "hello"})
	msg := cmd()
	result := msg.(compileResultMsg)
	if result.err != nil {
		t.Fatalf("compile error: %v", result.err)
	}
	if result.result.Error != "" {
		t.Fatalf("result error: %s", result.result.Error)
	}
	if !strings.Contains(result.result.Output, "[1] --verbose") {
		t.Errorf("output = %q, want to contain [1] --verbose", result.result.Output)
	}
	if !strings.Contains(result.result.Output, "[2] hello") {
		t.Errorf("output = %q, want to contain [2] hello", result.result.Output)
	}
}

func hasMake() bool {
	_, err := exec.LookPath("make")
	return err == nil
}

func testMultiFileLesson(id string, files []lesson.ExerciseFile, buildCmd, buildOutput string) lesson.Lesson {
	return lesson.Lesson{
		ID:                 id,
		Title:              "Multi-File Test",
		Exercise:           "",
		Files:              files,
		BuildCmd:           buildCmd,
		BuildOutput:        buildOutput,
		Language:           "c",
		SourceExtension:    ".c",
		SyntaxHighlighting: "c",
	}
}

func TestIntegration_MultiFileCompile(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	if !hasMake() {
		t.Skip("make not available")
	}

	files := []lesson.ExerciseFile{
		{
			Name:     "main.c",
			Editable: true,
			Language: "c",
			Content: `#include <stdio.h>
extern void helper_init(void);
int main(void) {
    helper_init();
    printf("[SERVER] Ready.\n");
    return 0;
}`,
		},
		{
			Name:     "helper.c",
			Editable: true,
			Language: "c",
			Content: `#include <stdio.h>
void helper_init(void) {
    printf("[HELPER] Booted.\n");
}`,
		},
		{
			Name:     "Makefile",
			Editable: true,
			Language: "makefile",
			Content: `CFLAGS=-Wall -Wextra -O0
TARGET=my-server
OBJS=main.o helper.o
$(TARGET): $(OBJS)
	gcc $(CFLAGS) -o $@ $^
%.o: %.c
	gcc $(CFLAGS) -c -o $@ $<
clean:
	rm -f $(TARGET) *.o
.PHONY: clean
`,
		},
	}

	l := testMultiFileLesson("test-multi-file", files, "make", "my-server")
	es := newTestExerciseScreen(t, l)

	// Multi-file: fileList should be created and currentFilesMap returns all files.
	fmap := es.currentFilesMap()
	if len(fmap) != 3 {
		t.Fatalf("currentFilesMap() returned %d files, want 3", len(fmap))
	}
	if _, ok := fmap["main.c"]; !ok {
		t.Error("currentFilesMap missing main.c")
	}
	if _, ok := fmap["helper.c"]; !ok {
		t.Error("currentFilesMap missing helper.c")
	}
	if _, ok := fmap["Makefile"]; !ok {
		t.Error("currentFilesMap missing Makefile")
	}

	// Compile using the custom build command.
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
		t.Fatalf("compile result error: %s", result.result.Error)
	}
	if !strings.Contains(result.result.Stdout, "[HELPER] Booted.") {
		t.Errorf("Stdout = %q, want to contain [HELPER] Booted.", result.result.Stdout)
	}
	if !strings.Contains(result.result.Stdout, "[SERVER] Ready.") {
		t.Errorf("Stdout = %q, want to contain [SERVER] Ready.", result.result.Stdout)
	}
}

func TestIntegration_DebugCompile(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	l := testLesson("test-dbg", ``, nil, nil)
	es := newTestExerciseScreen(t, l)

	code := `#include <stdio.h>
int main(void) {
    printf("debug me\n");
    return 0;
}`
	es.editor.SwitchFile(code, "c")

	cmd := es.gdbCompileCmd(nil)
	msg := cmd()
	result := msg.(gdbReadyMsg)
	if result.compileErr != nil && result.compileErr.Error != "" {
		t.Fatalf("compile error: %s", result.compileErr.Error)
	}
	if result.build == nil {
		t.Fatal("build is nil")
	}
	defer result.build.Close()
	if result.build.BinaryPath == "" {
		t.Error("BinaryPath is empty")
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
