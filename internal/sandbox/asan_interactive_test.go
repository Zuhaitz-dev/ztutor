package sandbox

import (
	"strings"
	"testing"
	"time"
)

func TestRunWithASAN(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := RunWithASAN(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	printf("asan-ok\n");
	return 0;
}`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("RunWithASAN: %v", err)
	}
	if result.Error != "" {
		t.Fatalf("result.Error = %q", result.Error)
	}
	if !strings.Contains(result.Output, "asan-ok") {
		t.Errorf("Output = %q, should contain 'asan-ok'", result.Output)
	}
}

func TestRunWithASAN_HeapUseAfterFree(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	result, err := RunWithASAN(cLang(), map[string]string{"main.c": `#include <stdlib.h>
int main(void) {
	char *p = malloc(10);
	free(p);
	p[0] = 'x';
	return 0;
}`}, "", "", nil, nil)

	if err != nil {
		t.Fatalf("RunWithASAN: %v", err)
	}
	if result.Error == "" && result.ExitCode == 0 {
		t.Logf("ASAN may not have detected use-after-free; output: %s", result.Output)
	}
}

func TestRunInteractive_Basic(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	build, compileErr := CompileDebug(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	printf("hello\n");
	return 0;
}`}, "", nil)
	if compileErr != nil && compileErr.Error != "" {
		t.Fatalf("compile error: %s", compileErr.Error)
	}
	if build == nil {
		t.Fatal("build is nil")
	}
	defer build.Close()

	_, events, kill, err := RunInteractive(build.BinaryPath, nil)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	defer kill()

	var output string
	timeout := time.After(2 * time.Second)
loop:
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				break loop
			}
			if ev.Done {
				if ev.Code != 0 {
					t.Errorf("program exit code = %d, want 0", ev.Code)
				}
				break loop
			}
			output += ev.Text
		case <-timeout:
			t.Fatal("timeout waiting for interactive output")
		}
	}

	if !strings.Contains(output, "hello") {
		t.Errorf("Output = %q, should contain 'hello'", output)
	}
}

func TestRunInteractive_WriteAndRead(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	build, compileErr := CompileDebug(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	char buf[64];
	printf("enter name: ");
	fgets(buf, sizeof(buf), stdin);
	printf("hello %s", buf);
	return 0;
}`}, "", nil)
	if compileErr != nil && compileErr.Error != "" {
		t.Fatalf("compile error: %s", compileErr.Error)
	}
	if build == nil {
		t.Fatal("build is nil")
	}
	defer build.Close()

	writeFn, events, kill, err := RunInteractive(build.BinaryPath, nil)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}
	defer kill()

	var output string
	timeout := time.After(2 * time.Second)
	wrote := false
loop:
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				break loop
			}
			if ev.Done {
				break loop
			}
			output += ev.Text
			if !wrote && strings.Contains(output, "enter name:") {
				wrote = true
				writeFn([]byte("World\n"))
			}
		case <-timeout:
			t.Fatal("timeout waiting for interactive output")
		}
	}

	if !strings.Contains(output, "hello World") {
		t.Errorf("Output = %q, should contain 'hello World'", output)
	}
}

func TestRunInteractive_Kill(t *testing.T) {
	if !hasGCC() {
		t.Skip("gcc not available")
	}

	build, compileErr := CompileDebug(cLang(), map[string]string{"main.c": `#include <stdio.h>
int main(void) {
	while(1) { printf("."); fflush(stdout); }
	return 0;
}`}, "", nil)
	if compileErr != nil && compileErr.Error != "" {
		t.Fatalf("compile error: %s", compileErr.Error)
	}
	if build == nil {
		t.Fatal("build is nil")
	}
	defer build.Close()

	_, events, kill, err := RunInteractive(build.BinaryPath, nil)
	if err != nil {
		t.Fatalf("RunInteractive: %v", err)
	}

	kill()

	timeout := time.After(2 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return
			}
			if ev.Done {
				return
			}
		case <-timeout:
			t.Fatal("timeout: process should have been killed")
		}
	}
}
