package sandbox

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func makeAvailable() bool {
	_, err := exec.LookPath("make")
	return err == nil
}

func TestBuildCmd_Make(t *testing.T) {
	if !hasGCC() || !makeAvailable() {
		t.Skip("gcc and/or make not available")
	}

	result, err := Run(cLang(), map[string]string{
		"main.c": `#include <stdio.h>
int main(void) {
	printf("make-built\n");
	return 0;
}`,
		"Makefile": `prog: main.c
	gcc -Wall -o prog main.c
`,
	}, "make", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if strings.TrimSpace(result.Output) != "make-built" {
		t.Errorf("Output = %q, want %q", result.Output, "make-built")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestBuildCmd_Make_Error(t *testing.T) {
	if !makeAvailable() {
		t.Skip("make not available")
	}

	result, err := Run(cLang(), map[string]string{
		"main.c": `int main(void) { return 0; }`,
		"Makefile": `prog: main.c
	gcc -Wall -o prog main.c
`,
	}, "make -f nonexistent.mk", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected build error, got none")
	}
}

func TestMultiFile_CompileAndRun(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := Run(cLang(), map[string]string{
		"main.c": `#include <stdio.h>
#include "utils.h"
int main(void) {
	printf("%s\n", greet());
	return 0;
}`,
		"utils.h": `#ifndef UTILS_H
#define UTILS_H
const char* greet(void);
#endif`,
		"utils.c": `#include "utils.h"
const char* greet(void) { return "multi-hello"; }`,
	}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q\noutput=%s", result.Error, result.Output)
	}
	if strings.TrimSpace(result.Output) != "multi-hello" {
		t.Errorf("Output = %q, want %q", result.Output, "multi-hello")
	}
	if result.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", result.ExitCode)
	}
}

func TestMultiFile_CompileError(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := Run(cLang(), map[string]string{
		"main.c": `#include "missing.h"
int main(void) { return 0; }`,
		"utils.c": `int helper(void) { return 1; }`,
	}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.Error == "" {
		t.Fatal("expected compilation error for missing header")
	}
}

func TestMultiFile_RunAllTests(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	code := `#include <stdio.h>
#include "utils.h"
int main(int argc, char *argv[]) {
	if (argc > 1)
		printf("%s %s\n", greet(), argv[1]);
	else
		printf("%s\n", greet());
	return 0;
}`

	tests := []TestInput{
		{Stdin: "", Args: nil, Expected: "hello from utils"},
		{Stdin: "", Args: []string{"world"}, Expected: "hello from utils world"},
	}

	compileRes, results, err := RunAllTests(cLang(), map[string]string{
		"main.c": code,
		"utils.h": `#ifndef UTILS_H
#define UTILS_H
const char* greet(void);
#endif`,
		"utils.c": `#include "utils.h"
const char* greet(void) { return "hello from utils"; }`,
	}, "", nil, tests)
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

func TestMultiFile_SafeWritePath(t *testing.T) {
	dir := t.TempDir()

	p, err := safeWritePath(dir, "main.c")
	if err != nil {
		t.Fatalf("safeWritePath(main.c): %v", err)
	}
	if p == "" {
		t.Error("safeWritePath should return a path")
	}

	_, err = safeWritePath(dir, "../etc/passwd")
	if err == nil {
		t.Error("safeWritePath should reject .. traversal")
	}

	_, err = safeWritePath(dir, "/etc/passwd")
	if err == nil {
		t.Error("safeWritePath should reject absolute paths")
	}

	_, err = safeWritePath(dir, "")
	if err == nil {
		t.Error("safeWritePath should reject empty names")
	}
}

func TestMultiFile_WriteFiles(t *testing.T) {
	dir := t.TempDir()

	err := writeFiles(dir, map[string]string{
		"main.c":      "// main",
		"lib/utils.c": "// utils",
	})
	if err != nil {
		t.Fatalf("writeFiles: %v", err)
	}

	for _, name := range []string{"main.c", "lib/utils.c"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Errorf("read %s: %v", name, err)
		}
		if string(data) == "" {
			t.Errorf("file %s is empty", name)
		}
	}
}
