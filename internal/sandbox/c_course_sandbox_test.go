package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func hasMake() bool {
	_, err := exec.LookPath("make")
	return err == nil
}

func TestMultiFileBuild_WithBuildCmd(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	if !hasMake() {
		t.Skip("make not available")
	}

	// Simulates lesson 14: multi-file build with "make" producing a
	// differently-named binary.  ensureProg() must rename it to "prog".
	files := map[string]string{
		"main.c": `#include <stdio.h>
extern void helper_init(void);
int main(void) {
    helper_init();
    printf("[SERVER] Ready.\n");
    return 0;
}`,
		"helper.c": `#include <stdio.h>
void helper_init(void) {
    printf("[HELPER] Booted.\n");
}`,
		"Makefile": `CFLAGS=-Wall -Wextra -O0
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
	}

	result, err := Run(cLang(), files, "make", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q (ensureProg may have failed)", result.Error)
	}
	if result.Stdout == "" && result.Stderr == "" {
		t.Fatal("no output from multi-file build execution")
	}
	if !strings.Contains(result.Stdout, "[HELPER] Booted.") {
		t.Errorf("Stdout missing [HELPER] Booted., got: %s", result.Stdout)
	}
	if !strings.Contains(result.Stdout, "[SERVER] Ready.") {
		t.Errorf("Stdout missing [SERVER] Ready., got: %s", result.Stdout)
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestMultiFileRunAllTests_WithBuildCmd(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	if !hasMake() {
		t.Skip("make not available")
	}

	files := map[string]string{
		"main.c": `#include <stdio.h>
int main(void) {
    printf("hello\n");
    return 0;
}`,
		"Makefile": `CFLAGS=-Wall -Wextra -O0
TARGET=my-prog
$(TARGET): main.c
	gcc $(CFLAGS) -o $@ $^
clean:
	rm -f $(TARGET)
.PHONY: clean
`,
	}

	tests := []TestInput{
		{Expected: "hello\n"},
	}

	compileRes, results, err := RunAllTests(cLang(), files, "make", nil, tests)
	if err != nil {
		t.Fatalf("RunAllTests: %v", err)
	}
	if compileRes.Error != "" {
		t.Fatalf("compile error: %s (ensureProg may have failed)", compileRes.Error)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	if !results[0].Passed {
		t.Errorf("test failed: got=%q want=%q", results[0].Got, results[0].Want)
	}
}

func TestMultiFileCompileDebug_WithBuildCmd(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}
	if !hasMake() {
		t.Skip("make not available")
	}

	files := map[string]string{
		"main.c": `int main(void) { return 0; }`,
		"Makefile": `TARGET=custom-bin
$(TARGET): main.c
	gcc -o $@ $^
clean:
	rm -f $(TARGET)
.PHONY: clean
`,
	}

	build, compileErr := CompileDebug(cLang(), files, "make", nil)
	if compileErr != nil && compileErr.Error != "" {
		t.Fatalf("CompileDebug error: %s", compileErr.Error)
	}
	if build == nil {
		t.Fatal("build is nil")
	}
	defer build.Close()

	if build.BinaryPath == "" {
		t.Fatal("BinaryPath is empty")
	}
	if !strings.HasSuffix(build.BinaryPath, "/prog") {
		t.Errorf("BinaryPath = %q, want .../prog (ensureProg should rename to prog)", build.BinaryPath)
	}
	if _, err := os.Stat(build.BinaryPath); err != nil {
		t.Errorf("BinaryPath %s does not exist: %v", build.BinaryPath, err)
	}
}

func TestBuildCmd_NonExistent(t *testing.T) {
	result, err := Run(cLang(), map[string]string{"main.c": `int main(void) { return 0; }`}, "nonexistent-binary-that-does-not-exist", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected error for non-existent build command")
	}
}

func TestRunAllTests_StreamAware(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <stdio.h>
int main(void) {
    fprintf(stderr, "debug: init\n");
    printf("payload\n");
    return 0;
}`

	tests := []TestInput{
		{
			ExpectedStdout:    "payload\n",
			ExpectedStderr:    "debug: init\n",
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
		t.Errorf("stream-aware test failed: got=%q want=%q err=%s", results[0].Got, results[0].Want, results[0].Error)
	}
}

func TestRunAllTests_ArgsPassthrough(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <stdio.h>
int main(int argc, char *argv[]) {
    for (int i = 1; i < argc; i++) {
        printf("arg[%d]=%s\n", i, argv[i]);
    }
    return 0;
}`

	tests := []TestInput{
		{Args: []string{"--verbose", "hello"}, Expected: "arg[1]=--verbose\narg[2]=hello\n"},
		{Args: nil, Expected: ""},
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
		t.Errorf("test 1 (with args): got=%q want=%q", results[0].Got, results[0].Want)
	}
	if !results[1].Passed {
		t.Errorf("test 2 (no args): got=%q want=%q", results[1].Got, results[1].Want)
	}
}

func TestEnsureProg_RenamesCustomBinary(t *testing.T) {
	dir := t.TempDir()

	// Create a custom-named binary.
	customPath := filepath.Join(dir, "my-server")
	if err := os.WriteFile(customPath, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatalf("write custom binary: %v", err)
	}

	progPath := filepath.Join(dir, "prog")
	if _, err := os.Stat(progPath); err == nil {
		t.Fatal("prog should not exist before ensureProg")
	}

	ensureProg(dir)

	if _, err := os.Stat(progPath); err != nil {
		t.Errorf("prog should exist after ensureProg, got: %v", err)
	}
	if _, err := os.Stat(customPath); err == nil {
		t.Error("custom binary should have been renamed away")
	}
}

func TestEnsureProg_NoOpWhenProgExists(t *testing.T) {
	dir := t.TempDir()

	progPath := filepath.Join(dir, "prog")
	if err := os.WriteFile(progPath, []byte("data"), 0644); err != nil {
		t.Fatalf("write prog: %v", err)
	}
	customPath := filepath.Join(dir, "other-bin")
	if err := os.WriteFile(customPath, []byte("data"), 0755); err != nil {
		t.Fatalf("write other: %v", err)
	}

	ensureProg(dir)

	if _, err := os.Stat(progPath); err != nil {
		t.Error("prog should still exist")
	}
	if _, err := os.Stat(customPath); err != nil {
		t.Error("other-bin should still exist (prog already existed)")
	}
}

func TestEnsureProg_SkipsDirectories(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "subdir")
	os.MkdirAll(subDir, 0755)

	progPath := filepath.Join(dir, "prog")
	if _, err := os.Stat(progPath); err == nil {
		t.Fatal("prog should not exist before ensureProg")
	}

	ensureProg(dir)

	if _, err := os.Stat(progPath); err == nil {
		t.Error("ensureProg should not create prog from a directory")
	}
}

func TestExecutionTimeout(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `int main(void) { for(;;) {} return 0; }`

	result, err := Run(cLang(), map[string]string{"main.c": code}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected timeout error for infinite loop")
	}
	if !strings.Contains(result.Error, "timed out") {
		t.Errorf("error = %q, want 'timed out'", result.Error)
	}
}

func TestSignal_SIGFPE(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	// CLONE_NEWPID makes the sandboxed process PID 1 inside the namespace.
	// PID 1 ignores unhandled signals by kernel policy, so we must install a
	// handler that calls _exit(128+sig) to ensure the process actually dies
	// and the sandbox can detect the crash.
	code := `#include <signal.h>
#include <stdlib.h>
#include <unistd.h>
void handler(int sig) { _exit(128 + sig); }
int main(void) { signal(SIGFPE, handler); raise(SIGFPE); return 0; }`

	result, err := Run(cLang(), map[string]string{"main.c": code}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" && result.ExitCode == 0 {
		t.Fatal("expected crash error for SIGFPE")
	}
	if !strings.Contains(result.Error, "FPE") && !strings.Contains(result.Error, "8") && result.ExitCode != 136 {
		t.Logf("SIGFPE: error=%q exit_code=%d", result.Error, result.ExitCode)
	}
}

func TestSignal_SIGSEGV(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <signal.h>
#include <stdlib.h>
#include <unistd.h>
void handler(int sig) { _exit(128 + sig); }
int main(void) { signal(SIGSEGV, handler); raise(SIGSEGV); return 0; }`

	result, err := Run(cLang(), map[string]string{"main.c": code}, "", "", nil, nil)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" && result.ExitCode == 0 {
		t.Fatal("expected crash error for null pointer dereference")
	}
	if result.Error != "" && !strings.Contains(result.Error, "crashed") {
		t.Logf("error = %q, exit code = %d (signal visible via exit code only)", result.Error, result.ExitCode)
	}
}

func TestRunAllTests_Signal_SIGSEGV(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <signal.h>
#include <stdlib.h>
#include <unistd.h>
void handler(int sig) { _exit(128 + sig); }
int main(void) { signal(SIGSEGV, handler); raise(SIGSEGV); return 0; }`

	tests := []TestInput{
		{Expected: ""},
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
	if results[0].Passed {
		t.Error("test should not pass (program crashes)")
	}
	if results[0].Error == "" && results[0].ExitCode == 0 {
		t.Error("expected crash error in test result")
	}
}

func TestRunAllTests_MultipleTestCases(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <stdio.h>
int main(void) {
    char buf[64];
    if (fgets(buf, sizeof(buf), stdin)) {
        printf("echo: %s", buf);
    }
    return 0;
}`

	tests := []TestInput{
		{Stdin: "hello\n", Expected: "echo: hello\n"},
		{Stdin: "world\n", Expected: "echo: world\n"},
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
		t.Errorf("test 1: got=%q want=%q", results[0].Got, results[0].Want)
	}
	if !results[1].Passed {
		t.Errorf("test 2: got=%q want=%q", results[1].Got, results[1].Want)
	}
}
