package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"ztutor/internal/logutil"
)

func init() {
	RegisterLanguage(&LanguageInfo{
		LangName:         "c",
		LangDisplayName:  "C",
		Extension:        ".c",
		Compiler:         "gcc",
		Debugger:         "gdb",
		Compiled:         true,
		HasDebuggerCap:   true,
		HasAsmCap:        true,
		HasSanCap:        true,
		DefFlags:         []string{"-Wall", "-Wextra", "-O0"},
		DbgFlags:         []string{"-g", "-O0"},
		AsmFlagsList:     []string{"-S", "-masm=intel", "-fno-asynchronous-unwind-tables", "-fno-pic"},
		SanFlags:         []string{"-g", "-O1", "-fsanitize=address,undefined", "-fno-omit-frame-pointer"},
		DebuggerArgsFn:   cDebuggerArgs,
		CompileFn:        cCompile,
		SyntaxCheckFn:    cSyntaxCheck,
		ExecuteFn:        cExecute,
		DebugBuildFn:     cCompileDebug,
		GenerateAsmFn:    cGenerateAssembly,
		CleanAsmFn:       cleanAssembly,
		ParseDiagFn:      parseGCCDiagnostics,
		InteractiveCmdFn: func(binaryPath string) (string, []string) { return binaryPath, nil },
	})
	RegisterLanguage(&LanguageInfo{
		LangName:         "cpp",
		LangDisplayName:  "C++",
		Extension:        ".cpp",
		Compiler:         "g++",
		Debugger:         "gdb",
		Compiled:         true,
		HasDebuggerCap:   true,
		HasAsmCap:        true,
		HasSanCap:        true,
		DefFlags:         []string{"-Wall", "-Wextra", "-O0", "-std=c++17"},
		DbgFlags:         []string{"-g", "-O0"},
		AsmFlagsList:     []string{"-S", "-masm=intel", "-fno-asynchronous-unwind-tables", "-fno-pic"},
		SanFlags:         []string{"-g", "-O1", "-fsanitize=address,undefined", "-fno-omit-frame-pointer"},
		DebuggerArgsFn:   cDebuggerArgs,
		CompileFn:        cCompile,
		SyntaxCheckFn:    cSyntaxCheck,
		ExecuteFn:        cExecute,
		DebugBuildFn:     cCompileDebug,
		GenerateAsmFn:    cGenerateAssembly,
		CleanAsmFn:       cleanAssembly,
		ParseDiagFn:      parseGCCDiagnostics,
		InteractiveCmdFn: func(binaryPath string) (string, []string) { return binaryPath, nil },
	})
}

func cDebuggerArgs(binaryPath string) []string {
	return []string{"-q", "-iex", "set debuginfod enabled off", "--args", binaryPath}
}

func cCompile(dir, srcPath string, flags []string, compiler string) *Result {
	outPath := filepath.Join(dir, "prog")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := append([]string{"-Wall", "-Wextra", "-O0"}, flags...)
	args = append(args, "-o", outPath, srcPath)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return &Result{Error: "compilation timed out"}
		}
		return &Result{Error: fmt.Sprintf("compilation error:\n%s", stripDir(stderr.String(), dir))}
	}

	if err := os.Chmod(dir, 0755); err != nil {
		logutil.Warn("sandbox: chmod dir: %v", err)
	}
	if err := os.Chmod(outPath, 0755); err != nil {
		logutil.Warn("sandbox: chmod binary: %v", err)
	}
	return &Result{Output: stripDir(stderr.String(), dir)}
}

func cSyntaxCheck(dir, srcPath string, flags []string, compiler string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"-fsyntax-only", "-Wall", "-Wextra", "-fdiagnostics-color=never"}
	args = append(args, flags...)
	args = append(args, srcPath)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			return &Result{Error: fmt.Sprintf("syntax check failed: %v", err)}
		}
	}

	return &Result{Output: stderr.String()}
}

func parseGCCDiagnostics(output string) []Diagnostic {
	diagRE := regexp.MustCompile(`^[^:]+:(\d+):(\d+): (error|warning|note): (.+)$`)
	var diags []Diagnostic
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		m := diagRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		key := fmt.Sprintf("%d:%d:%s", lineNum, col, m[3])
		if seen[key] {
			continue
		}
		seen[key] = true
		diags = append(diags, Diagnostic{
			Line:    lineNum,
			Col:     col,
			Kind:    m[3],
			Message: m[4],
		})
	}
	return diags
}

func cExecute(dir, stdin string, runtimeArgs, extraEnv []string, compiler string) (*Result, error) {
	progPath := filepath.Join(dir, "prog")
	return executeBinary(progPath, dir, stdin, runtimeArgs, extraEnv)
}

func cCompileDebug(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result) {
	debugFlags := append([]string{"-g", "-O0"}, flags...)
	result := cCompile(dir, srcPath, debugFlags, compiler)
	if result.Error != "" {
		return nil, result
	}
	return &DebugBuild{BinaryPath: filepath.Join(dir, "prog"), dir: dir}, nil
}

func cGenerateAssembly(dir, srcPath string, flags []string, compiler string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := append([]string{"-S", "-masm=intel", "-fno-asynchronous-unwind-tables", "-fno-pic", "-Wall", "-Wextra"}, flags...)
	args = append(args, "-o", "/dev/stdout", srcPath)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("compilation timed out")
		}
		return "", fmt.Errorf("compilation error:\n%s", stripDir(stderr.String(), dir))
	}

	return cleanAssembly(stripDir(stdout.String(), dir)), nil
}

// cleanAssembly strips CFI directives and metadata noise from gcc -S output.
func cleanAssembly(asm string) string {
	var out []string
	prevBlank := false
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, ".cfi_") ||
			strings.HasPrefix(trimmed, ".file") ||
			strings.HasPrefix(trimmed, ".ident") ||
			strings.HasPrefix(trimmed, ".note") ||
			strings.HasPrefix(trimmed, ".size") {
			continue
		}
		blank := trimmed == ""
		if blank && prevBlank {
			continue
		}
		out = append(out, line)
		prevBlank = blank
	}
	return strings.Join(out, "\n")
}

// ── Shared execution ──────────────────────────────────────────────────────────

func executeBinary(progPath, dir, stdin string, runtimeArgs, extraEnv []string) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Limits.MaxRuntime)
	defer cancel()

	cmd := exec.CommandContext(ctx, progPath, runtimeArgs...)
	cmd.Dir = dir
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	cmd.Env = sandboxEnv(dir, extraEnv)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	cmd.SysProcAttr = executionSysProcAttr()

	var err error
	if canUseNamespaces {
		setNamespaceOpts(cmd)
		limits := setResourceLimits()
		err = cmd.Run()
		restoreResourceLimits(limits)
	} else {
		err = cmd.Run()
	}

	return formatExecuteResult(stdout.String(), stderr.String(), dir, ctx.Err(), err), nil
}
