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
		LangName:        "go",
		LangDisplayName: "Go",
		Extension:       ".go",
		Compiler:        "go",
		Debugger:        "dlv",
		Compiled:        true,
		HasDebuggerCap:  true,
		HasAsmCap:       true,
		HasSanCap:       false,
		DefFlags:        nil,
		DbgFlags:        nil,
		AsmFlagsList:    nil,
		DebuggerArgsFn:  goDebuggerArgs,
		CompileFn:       goCompile,
		SyntaxCheckFn:   goSyntaxCheck,
		ExecuteFn:       cExecute,
		DebugBuildFn:    goCompileDebug,
		GenerateAsmFn:   goGenerateAssembly,
		CleanAsmFn:      cleanGoAssembly,
		ParseDiagFn:     parseGoDiagnostics,
	})

	RegisterLanguage(&LanguageInfo{
		LangName:         "ruby",
		LangDisplayName:  "Ruby",
		Extension:        ".rb",
		Compiler:         "ruby",
		Debugger:         "byebug",
		Compiled:         false,
		HasDebuggerCap:   true,
		HasAsmCap:        false,
		HasSanCap:        false,
		DefFlags:         nil,
		DbgFlags:         nil,
		DebuggerArgsFn:   rubyDebuggerArgs,
		SyntaxCheckFn:    rubySyntaxCheck,
		ExecuteFn:        rubyExecute,
		DebugBuildFn:     rubyDebugBuild,
		ParseDiagFn:      parseRubyDiagnostics,
		InteractiveCmdFn: rubyInteractiveCmd,
	})

	RegisterLanguage(&LanguageInfo{
		LangName:         "java",
		LangDisplayName:  "Java",
		Extension:        ".java",
		Compiler:         "javac",
		Debugger:         "jdb",
		Compiled:         true,
		HasDebuggerCap:   true,
		HasAsmCap:        false,
		HasSanCap:        false,
		DefFlags:         nil,
		DbgFlags:         []string{"-g"},
		DebuggerArgsFn:   javaDebuggerArgs,
		CompileFn:        javaCompile,
		SyntaxCheckFn:    javaSyntaxCheck,
		ExecuteFn:        javaExecute,
		DebugBuildFn:     javaCompileDebug,
		ParseDiagFn:      parseJavaDiagnostics,
		InteractiveCmdFn: javaInteractiveCmd,
	})
}

// ── Go ────────────────────────────────────────────────────────────────────────

func goDebuggerArgs(binaryPath string) []string {
	return []string{"exec", binaryPath}
}

func goCompile(dir, srcPath string, flags []string, compiler string) *Result {
	outPath := filepath.Join(dir, "prog")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"build", "-o", outPath}
	args = append(args, flags...)
	args = append(args, srcPath)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return &Result{Error: "compilation timed out"}
		}
		return &Result{Error: fmt.Sprintf("compilation error:\n%s", stripDir(stderr.String(), dir))}
	}

	if err := os.Chmod(outPath, 0755); err != nil {
		logutil.Warn("sandbox: chmod binary: %v", err)
	}
	return &Result{Output: stripDir(stderr.String(), dir)}
}

func goSyntaxCheck(dir, srcPath string, flags []string, compiler string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"build", "-o", "/dev/null"}
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

func goCompileDebug(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result) {
	debugFlags := append([]string{"-gcflags", "all=-N -l"}, flags...)
	result := goCompile(dir, srcPath, debugFlags, compiler)
	if result.Error != "" {
		return nil, result
	}
	return &DebugBuild{BinaryPath: filepath.Join(dir, "prog"), dir: dir}, nil
}

func goGenerateAssembly(dir, srcPath string, flags []string, compiler string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"build", "-gcflags", "-S", "-o", "/dev/null"}
	args = append(args, flags...)
	args = append(args, srcPath)

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

	output := stdout.String()
	if output == "" {
		output = stderr.String()
	}
	return cleanGoAssembly(stripDir(output, dir)), nil
}

func cleanGoAssembly(asm string) string {
	var out []string
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}

func parseGoDiagnostics(output string) []Diagnostic {
	re := regexp.MustCompile(`^[^:]+:(\d+):(\d+): (.+)$`)
	var diags []Diagnostic
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[1])
		col, _ := strconv.Atoi(m[2])
		msg := m[3]
		key := fmt.Sprintf("%d:%d:%s", lineNum, col, msg)
		if seen[key] {
			continue
		}
		seen[key] = true
		kind := "error"
		if strings.HasPrefix(msg, "warning:") {
			kind = "warning"
		}
		diags = append(diags, Diagnostic{
			Line:    lineNum,
			Col:     col,
			Kind:    kind,
			Message: msg,
		})
	}
	return diags
}

// ── Ruby ──────────────────────────────────────────────────────────────────────

func rubyDebuggerArgs(binaryPath string) []string {
	return []string{"-r", "byebug", binaryPath}
}

func rubyInteractiveCmd(binaryPath string) (string, []string) {
	return "ruby", []string{binaryPath}
}

func rubySyntaxCheck(dir, srcPath string, flags []string, compiler string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	args := []string{"-c", srcPath}
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

func rubyExecute(dir, stdin string, runtimeArgs, extraEnv []string, compiler string) (*Result, error) {
	progPath := filepath.Join(dir, "main.rb")
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

func rubyDebugBuild(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result) {
	return &DebugBuild{BinaryPath: srcPath, dir: dir}, nil
}

func parseRubyDiagnostics(output string) []Diagnostic {
	re := regexp.MustCompile(`^[^:]+:(\d+): (.+)$`)
	var diags []Diagnostic
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[1])
		msg := m[2]
		key := fmt.Sprintf("%d:%s", lineNum, msg)
		if seen[key] {
			continue
		}
		seen[key] = true
		kind := "error"
		if strings.HasPrefix(msg, "warning:") {
			kind = "warning"
		}
		diags = append(diags, Diagnostic{
			Line:    lineNum,
			Kind:    kind,
			Message: msg,
		})
	}
	return diags
}

// ── Java ──────────────────────────────────────────────────────────────────────

func javaDebuggerArgs(binaryPath string) []string {
	return []string{"-classpath", filepath.Dir(binaryPath), "Main"}
}

func javaInteractiveCmd(binaryPath string) (string, []string) {
	return "java", []string{"-cp", filepath.Dir(binaryPath), "Main"}
}

func ensureJavaFilename(dir, srcPath string) string {
	if filepath.Base(srcPath) == "main.java" {
		javaPath := filepath.Join(dir, "Main.java")
		if _, err := os.Stat(srcPath); err == nil {
			if err := os.Rename(srcPath, javaPath); err != nil {
				logutil.Warn("sandbox: rename java file: %v", err)
				return srcPath
			}
			return javaPath
		}
	}
	return srcPath
}

func javaCompile(dir, srcPath string, flags []string, compiler string) *Result {
	srcPath = ensureJavaFilename(dir, srcPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"-d", dir, srcPath}
	args = append(args, flags...)

	cmd := exec.CommandContext(ctx, compiler, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return &Result{Error: "compilation timed out"}
		}
		return &Result{Error: fmt.Sprintf("compilation error:\n%s", stripDir(stderr.String(), dir))}
	}

	return &Result{Output: stripDir(stderr.String(), dir)}
}

func javaSyntaxCheck(dir, srcPath string, flags []string, compiler string) *Result {
	srcPath = ensureJavaFilename(dir, srcPath)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	args := []string{"-d", dir, srcPath}
	args = append(args, flags...)

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

func javaExecute(dir, stdin string, runtimeArgs, extraEnv []string, compiler string) (*Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), Limits.MaxRuntime)
	defer cancel()

	args := append([]string{"-cp", dir, "Main"}, runtimeArgs...)
	cmd := exec.CommandContext(ctx, "java", args...)
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

func javaCompileDebug(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result) {
	debugFlags := append([]string{"-g"}, flags...)
	result := javaCompile(dir, srcPath, debugFlags, compiler)
	if result.Error != "" {
		return nil, result
	}
	return &DebugBuild{BinaryPath: filepath.Join(dir, "Main.class"), dir: dir}, nil
}

func parseJavaDiagnostics(output string) []Diagnostic {
	re := regexp.MustCompile(`^[^:]+:(\d+): (error|warning): (.+)$`)
	var diags []Diagnostic
	seen := map[string]bool{}
	for _, line := range strings.Split(output, "\n") {
		m := re.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lineNum, _ := strconv.Atoi(m[1])
		kind := m[2]
		msg := m[3]
		key := fmt.Sprintf("%d:%d:%s", lineNum, 0, msg)
		if seen[key] {
			continue
		}
		seen[key] = true
		diags = append(diags, Diagnostic{
			Line:    lineNum,
			Kind:    kind,
			Message: msg,
		})
	}
	return diags
}
