package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"ztutor/internal/logutil"
)

func init() {
	RegisterLanguage(&LanguageInfo{
		LangName:         "python",
		LangDisplayName:  "Python",
		Extension:        ".py",
		Compiler:         "python3",
		Debugger:         "pdb3",
		Compiled:         false,
		HasDebuggerCap:   true,
		HasAsmCap:        false,
		HasSanCap:        false,
		DefFlags:         nil,
		DbgFlags:         nil,
		DebuggerArgsFn:   pythonDebuggerArgs,
		SyntaxCheckFn:    pythonSyntaxCheck,
		ExecuteFn:        pythonExecute,
		DebugBuildFn:     pythonDebugBuild,
		ParseDiagFn:      parsePythonDiagnostics,
		InteractiveCmdFn: pythonInteractiveCmd,
	})
	RegisterLanguage(&LanguageInfo{
		LangName:         "rust",
		LangDisplayName:  "Rust",
		Extension:        ".rs",
		Compiler:         "rustc",
		Debugger:         "rust-gdb",
		Compiled:         true,
		HasDebuggerCap:   true,
		HasAsmCap:        true,
		HasSanCap:        false,
		DefFlags:         []string{"-C", "opt-level=0"},
		DbgFlags:         []string{"-g", "-C", "opt-level=0"},
		AsmFlagsList:     []string{"--emit", "asm", "-C", "opt-level=0"},
		DebuggerArgsFn:   cDebuggerArgs,
		CompileFn:        rustCompile,
		SyntaxCheckFn:    rustCheck,
		ExecuteFn:        cExecute,
		DebugBuildFn:     rustCompileDebug,
		GenerateAsmFn:    rustGenerateAssembly,
		CleanAsmFn:       cleanAssembly,
		ParseDiagFn:      parseGCCDiagnostics,
		InteractiveCmdFn: func(binaryPath string) (string, []string) { return binaryPath, nil },
	})
}

func pythonInteractiveCmd(binaryPath string) (string, []string) {
	return "python3", []string{binaryPath}
}

func pythonDebuggerArgs(binaryPath string) []string {
	return []string{"-m", "pdb", binaryPath}
}

func pythonSyntaxCheck(dir, srcPath string, flags []string, compiler string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"-m", "py_compile", srcPath}
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

func parsePythonDiagnostics(output string) []Diagnostic {
	var diags []Diagnostic
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Python format: "  File \"main.py\", line 3" or "SyntaxError: invalid syntax"
		if strings.HasPrefix(line, "  File ") {
			parts := strings.Split(line, "line ")
			if len(parts) == 2 {
				numStr := strings.TrimSuffix(strings.TrimSpace(parts[1]), ",")
				if n, err := strconv.Atoi(numStr); err == nil {
					diags = append(diags, Diagnostic{Line: n, Kind: "error", Message: "see below"})
				}
			}
		} else if strings.Contains(line, "Error") || strings.Contains(line, "error") {
			msg := line
			// Use the line number from the previous File entry if available.
			if len(diags) > 0 {
				diags[len(diags)-1].Message = msg
			} else {
				diags = append(diags, Diagnostic{Line: 1, Kind: "error", Message: msg})
			}
		}
	}
	return diags
}

func pythonExecute(dir, stdin string, runtimeArgs, extraEnv []string, compiler string) (*Result, error) {
	progPath := filepath.Join(dir, "main.py")
	ctx, cancel := context.WithTimeout(context.Background(), Limits.MaxRuntime)
	defer cancel()

	args := append([]string{progPath}, runtimeArgs...)
	cmd := exec.CommandContext(ctx, compiler, args...)
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

func pythonDebugBuild(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result) {
	// Python doesn't compile; return the source as the "binary"
	return &DebugBuild{BinaryPath: srcPath, dir: dir}, nil
}

func rustCompile(dir, srcPath string, flags []string, compiler string) *Result {
	outPath := filepath.Join(dir, "prog")
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	args := []string{"-C", "opt-level=0"}
	args = append(args, flags...)
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

func rustCheck(dir, srcPath string, flags []string, compiler string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"--edition", "2021", "--emit", "metadata", srcPath}
	args = append(args, flags...)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Run()

	return &Result{Output: stderr.String()}
}

func rustCompileDebug(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result) {
	debugFlags := append([]string{"-g", "-C", "opt-level=0"}, flags...)
	result := rustCompile(dir, srcPath, debugFlags, compiler)
	if result.Error != "" {
		return nil, result
	}
	return &DebugBuild{BinaryPath: filepath.Join(dir, "prog"), dir: dir}, nil
}

func rustGenerateAssembly(dir, srcPath string, flags []string, compiler string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"--emit", "asm", "-C", "opt-level=0"}
	args = append(args, flags...)
	args = append(args, "-o", filepath.Join(dir, "out"), srcPath)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return "", fmt.Errorf("compilation timed out")
		}
		return "", fmt.Errorf("compilation error:\n%s", stripDir(stderr.String(), dir))
	}

	asmPath := filepath.Join(dir, "out.s")
	data, err := os.ReadFile(asmPath)
	if err != nil {
		return "", fmt.Errorf("read assembly: %w", err)
	}

	return cleanAssembly(stripDir(string(data), dir)), nil
}
