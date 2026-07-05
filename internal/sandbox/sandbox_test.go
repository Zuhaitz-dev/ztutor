package sandbox

import (
	"os/exec"
	"strings"
	"testing"
	"time"
)

func cLang() Language {
	return GetLanguage("c")
}

func hasGCC() bool {
	_, err := exec.LookPath("gcc")
	return err == nil
}

func TestCompileAndRun(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := Run(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	printf("hello\n");
	return 0;
}`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Stdout) != "hello" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello")
	}
	if result.Stderr != "" {
		t.Errorf("Stderr = %q, want empty", result.Stderr)
	}
	if strings.TrimSpace(result.Output) != "hello" {
		t.Errorf("Output = %q, want %q", result.Output, "hello")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestCompileError(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := Run(cLang(), map[string]string{"main.c": `ints main(void) { return 0; }`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected compilation error")
	}
}

func TestOutputWithStdin(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := Run(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	char buf[64];
	fgets(buf, sizeof(buf), stdin);
	printf("you said: %s", buf);
	return 0;
}`}, "", "hello\n", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "you said: hello" {
		t.Errorf("Output = %q", result.Output)
	}
}

func TestExitCode(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := Run(cLang(), map[string]string{"main.c": `int main(void) { return 42; }`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if result.ExitCode != 42 {
		t.Errorf("ExitCode = %d, want 42", result.ExitCode)
	}
}

func TestRunAllTests(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <stdio.h>
int main(int argc, char *argv[]) {
	if (argc > 1) {
		printf("arg: %s\n", argv[1]);
	}
	char buf[64];
	if (fgets(buf, sizeof(buf), stdin)) {
		printf("input: %s", buf);
	}
	return 0;
}`

	tests := []TestInput{
		{Stdin: "hello\n", Args: nil, Expected: "input: hello"},
		{Stdin: "world\n", Args: []string{"test"}, Expected: "arg: test\ninput: world"},
	}

	compileRes, results, err := RunAllTests(cLang(), map[string]string{"main.c": code}, "", nil, tests)
	if err != nil {
		t.Fatalf("RunAllTests: %v", err)
	}
	if compileRes.Error != "" {
		t.Fatalf("compile error: %s", compileRes.Error)
	}
	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}
	if !results[0].Passed {
		t.Errorf("test 1 failed: got=%q want=%q err=%s", results[0].Got, results[0].Want, results[0].Error)
	}
	if !results[1].Passed {
		t.Errorf("test 2 failed: got=%q want=%q err=%s", results[1].Got, results[1].Want, results[1].Error)
	}
}

func TestRunAllTests_StreamAwareExpectations(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <stdio.h>
int main(void) {
	fprintf(stderr, "debug\n");
	printf("payload\n");
	return 0;
}`

	tests := []TestInput{
		{
			ExpectedStdout:    "payload\n",
			ExpectedStderr:    "debug\n",
			HasExpectedStdout: true,
			HasExpectedStderr: true,
		},
	}

	compileRes, results, err := RunAllTests(cLang(), map[string]string{"main.c": code}, "", nil, tests)
	if err != nil {
		t.Fatalf("RunAllTests: %v", err)
	}
	if compileRes.Error != "" {
		t.Fatalf("compile error: %s", compileRes.Error)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if !results[0].Passed {
		t.Fatalf("stream-aware test failed: got=%q want=%q err=%s", results[0].Got, results[0].Want, results[0].Error)
	}
}

func TestSyntaxCheck(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	diags, err := SyntaxCheck(cLang(), map[string]string{"main.c": `int main(void) {
		invalid syntax here
		return 0;
	}`}, "", nil)

	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	if len(diags) == 0 {
		t.Fatal("expected diagnostic(s)")
	}
	found := false
	for _, d := range diags {
		if d.Kind == "error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected error diagnostic, got %v", diags)
	}
}

func TestSyntaxCheck_Clean(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	diags, err := SyntaxCheck(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	printf("hello\n");
	return 0;
}`}, "", nil)

	if err != nil {
		t.Fatalf("SyntaxCheck: %v", err)
	}
	for _, d := range diags {
		if d.Kind == "error" {
			t.Errorf("unexpected error: %s", d.Message)
		}
	}
}

func TestGenerateAssembly(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	asm, err := GenerateAssembly(cLang(), map[string]string{"main.c": `int add(int a, int b) { return a + b; }`}, "", nil)
	if err != nil {
		t.Fatalf("GenerateAssembly: %v", err)
	}
	if asm == "" {
		t.Fatal("empty assembly output")
	}
	if !strings.Contains(asm, "add") {
		t.Error("assembly should contain function name")
	}
}

func TestCompileDebug(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	build, compileErr := CompileDebug(cLang(), map[string]string{"main.c": `int main(void) { return 0; }`}, "", nil)
	if compileErr != nil && compileErr.Error != "" {
		t.Fatalf("compile error: %s", compileErr.Error)
	}
	if build == nil {
		t.Fatal("build is nil")
	}
	defer build.Close()

	if build.BinaryPath == "" {
		t.Error("BinaryPath is empty")
	}
}

func TestCompileDebug_Error(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	build, compileErr := CompileDebug(cLang(), map[string]string{"main.c": `invalid C code {{{`}, "", nil)
	if compileErr == nil || compileErr.Error == "" {
		if build != nil {
			build.Close()
		}
		t.Fatal("expected compilation error")
	}
}

func TestStripDir(t *testing.T) {
	result := stripDir("/tmp/ztutor-sandbox-abc/main.c:4: error: foo", "/tmp/ztutor-sandbox-abc")
	if !strings.Contains(result, "main.c:4:") {
		t.Errorf("stripDir = %q, should contain main.c:4:", result)
	}
	if strings.Contains(result, "/tmp/ztutor-sandbox-abc") {
		t.Errorf("stripDir should not contain temp dir, got %q", result)
	}
}

func TestParseDiagnostics(t *testing.T) {
	output := "main.c:5:10: error: expected ';' before 'return'\nmain.c:3:1: warning: unused variable 'x'\n"
	lang := GetLanguage("c")
	if lang == nil {
		t.Fatal("GetLanguage('c') returned nil")
	}
	diags := lang.ParseDiagnostics(output)

	if len(diags) != 2 {
		t.Fatalf("diags = %d, want 2", len(diags))
	}
	if diags[0].Kind != "error" || diags[0].Line != 5 || diags[0].Col != 10 {
		t.Errorf("diag[0] = %+v", diags[0])
	}
	if diags[1].Kind != "warning" || diags[1].Line != 3 || diags[1].Col != 1 {
		t.Errorf("diag[1] = %+v", diags[1])
	}
}

func TestGetLanguage(t *testing.T) {
	lang := GetLanguage("c")
	if lang == nil {
		t.Fatal("GetLanguage('c') returned nil")
	}
	if lang.Name() != "c" {
		t.Errorf("Name() = %q, want 'c'", lang.Name())
	}
}

func TestRunDebugger_ProcessLifecycle(t *testing.T) {
	if _, err := exec.LookPath("cat"); err != nil {
		t.Skip("cat not available")
	}

	writeFn, events, kill, err := RunDebugger("cat", nil)
	if err != nil {
		t.Fatalf("RunDebugger: %v", err)
	}
	defer func() {
		if kill != nil {
			kill()
		}
	}()

	if writeFn == nil {
		t.Fatal("writeFn is nil")
	}

	if err := writeFn([]byte("hello\n")); err != nil {
		t.Fatalf("writeFn: %v", err)
	}

	select {
	case ev, ok := <-events:
		if !ok {
			t.Error("events channel closed unexpectedly")
		}
		if !strings.Contains(ev.Text, "hello") {
			t.Errorf("expected output to contain 'hello', got %q", ev.Text)
		}
		if ev.Done {
			t.Error("expected not done yet")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for output")
	}

	kill()

	select {
	case ev, ok := <-events:
		if ok && ev.Done {
			// Success: process ended and sent done.
		} else if !ok {
			t.Error("events channel closed without done event")
		} else {
			t.Error("expected done event after kill")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for done event after kill")
	}
}
