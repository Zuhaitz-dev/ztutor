package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"ztutor/internal/logutil"
)

type Language interface {
	Name() string
	DisplayName() string
	SourceExtension() string // e.g. ".c", ".py"
	SourceFileName() string  // e.g. "main.c", "main.py"

	// Execution model
	IsCompiled() bool // true for C/C++/Rust, false for Python/Ruby/JS

	// Compilation / execution
	Compile(dir, srcPath string, flags []string) *Result
	// CompileFiles compiles multiple source files into one binary at dir/prog.
	// Single-file: delegates to Compile. Multi-file: invokes compiler directly.
	CompileFiles(dir string, srcPaths []string, flags []string) *Result
	CheckSyntax(dir, srcPath string, flags []string) *Result
	Execute(dir, stdin string, runtimeArgs, extraEnv []string) (*Result, error)

	// Diagnostics parsing (per-language error format)
	ParseDiagnostics(output string) []Diagnostic

	// Debugging
	HasDebugger() bool
	DebuggerPath() string
	DebuggerArgs(binaryPath string) []string
	CompileDebug(dir, srcPath string, flags []string) (*DebugBuild, *Result)

	// Assembly view
	HasAssembly() bool
	AsmFlags() []string
	GenerateAssembly(dir, srcPath string, flags []string) (string, error)

	// Sanitizers / extra tools
	HasSanitizers() bool
	SanitizerFlags() []string

	// Interactive mode
	InteractiveCommand(binaryPath string) (cmd string, args []string) // for non-compiled: returns interpreter + source

	// Compiler flags
	DefaultFlags() []string
	DebugFlags() []string
}

type LanguageInfo struct {
	LangName         string
	LangDisplayName  string
	Extension        string
	Compiler         string // resolved path or name
	Debugger         string
	Compiled         bool
	HasDebuggerCap   bool
	HasAsmCap        bool
	HasSanCap        bool
	DefFlags         []string
	DbgFlags         []string
	AsmFlagsList     []string
	SanFlags         []string
	DebuggerArgsFn   func(string) []string
	CompileFn        func(dir, srcPath string, flags []string, compiler string) *Result
	SyntaxCheckFn    func(dir, srcPath string, flags []string, compiler string) *Result
	ExecuteFn        func(dir, stdin string, runtimeArgs, extraEnv []string, compiler string) (*Result, error)
	DebugBuildFn     func(dir, srcPath string, flags []string, compiler string) (*DebugBuild, *Result)
	GenerateAsmFn    func(dir, srcPath string, flags []string, compiler string) (string, error)
	CleanAsmFn       func(string) string
	ParseDiagFn      func(string) []Diagnostic
	InteractiveCmdFn func(binaryPath string) (string, []string)

	compilerPath string
	debuggerPath string
}

func (l *LanguageInfo) Name() string             { return l.LangName }
func (l *LanguageInfo) DisplayName() string      { return l.LangDisplayName }
func (l *LanguageInfo) SourceExtension() string  { return l.Extension }
func (l *LanguageInfo) SourceFileName() string   { return "main" + l.Extension }
func (l *LanguageInfo) IsCompiled() bool         { return l.Compiled }
func (l *LanguageInfo) HasDebugger() bool        { return l.HasDebuggerCap }
func (l *LanguageInfo) DebuggerPath() string     { return l.debuggerPath }
func (l *LanguageInfo) HasAssembly() bool        { return l.HasAsmCap }
func (l *LanguageInfo) HasSanitizers() bool      { return l.HasSanCap }
func (l *LanguageInfo) DefaultFlags() []string   { return l.DefFlags }
func (l *LanguageInfo) DebugFlags() []string     { return l.DbgFlags }
func (l *LanguageInfo) AsmFlags() []string       { return l.AsmFlagsList }
func (l *LanguageInfo) SanitizerFlags() []string { return l.SanFlags }

func (l *LanguageInfo) ParseDiagnostics(output string) []Diagnostic {
	if l.ParseDiagFn != nil {
		return l.ParseDiagFn(output)
	}
	return nil
}

func (l *LanguageInfo) InteractiveCommand(binaryPath string) (string, []string) {
	if l.InteractiveCmdFn != nil {
		return l.InteractiveCmdFn(binaryPath)
	}
	return binaryPath, nil
}

func (l *LanguageInfo) DebuggerArgs(binaryPath string) []string {
	if l.DebuggerArgsFn != nil {
		return l.DebuggerArgsFn(binaryPath)
	}
	return nil
}

func (l *LanguageInfo) Compile(dir, srcPath string, flags []string) *Result {
	if l.CompileFn != nil {
		return l.CompileFn(dir, srcPath, flags, l.compilerPath)
	}
	return &Result{Error: "compilation not supported for " + l.Name()}
}

func (l *LanguageInfo) CompileFiles(dir string, srcPaths []string, flags []string) *Result {
	if len(srcPaths) == 0 {
		return &Result{Error: "no source files to compile"}
	}
	if len(srcPaths) == 1 {
		return l.Compile(dir, srcPaths[0], flags)
	}
	if !l.Compiled {
		return &Result{Error: "multi-file compilation not supported for interpreted language " + l.Name()}
	}
	if l.compilerPath == "" {
		return &Result{Error: "compiler not found for " + l.Name()}
	}
	outPath := filepath.Join(dir, "prog")
	ctx, cancel := context.WithTimeout(context.Background(), Limits.MaxCompileRuntime)
	defer cancel()
	args := append([]string{}, flags...)
	args = append(args, "-o", outPath)
	args = append(args, srcPaths...)
	cmd := exec.CommandContext(ctx, l.compilerPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return &Result{Error: "compilation timed out"}
		}
		out := stderr.String()
		if out == "" {
			out = err.Error()
		}
		return &Result{Error: out}
	}
	return &Result{}
}

func (l *LanguageInfo) CheckSyntax(dir, srcPath string, flags []string) *Result {
	if l.SyntaxCheckFn != nil {
		return l.SyntaxCheckFn(dir, srcPath, flags, l.compilerPath)
	}
	return &Result{}
}

func (l *LanguageInfo) Execute(dir, stdin string, runtimeArgs, extraEnv []string) (*Result, error) {
	if l.ExecuteFn != nil {
		return l.ExecuteFn(dir, stdin, runtimeArgs, extraEnv, l.compilerPath)
	}
	return nil, fmt.Errorf("execution not supported for " + l.Name())
}

func (l *LanguageInfo) CompileDebug(dir, srcPath string, flags []string) (*DebugBuild, *Result) {
	if l.DebugBuildFn != nil {
		return l.DebugBuildFn(dir, srcPath, flags, l.compilerPath)
	}
	return nil, &Result{Error: "debug compilation not supported for " + l.Name()}
}

func (l *LanguageInfo) GenerateAssembly(dir, srcPath string, flags []string) (string, error) {
	if l.GenerateAsmFn != nil {
		return l.GenerateAsmFn(dir, srcPath, flags, l.compilerPath)
	}
	return "", fmt.Errorf("assembly view not supported for " + l.Name())
}

var languages = map[string]*LanguageInfo{}

func RegisterLanguage(info *LanguageInfo) {
	if info.compilerPath == "" && info.Compiler != "" {
		if p, err := exec.LookPath(info.Compiler); err == nil {
			info.compilerPath = p
		} else {
			logutil.Warn("%s compiler (%s) not found — %s programs will not compile", info.LangDisplayName, info.Compiler, info.LangDisplayName)
			info.compilerPath = info.Compiler
		}
	}
	if info.debuggerPath == "" && info.Debugger != "" {
		if p, err := exec.LookPath(info.Debugger); err == nil {
			info.debuggerPath = p
		} else {
			info.debuggerPath = info.Debugger
		}
	}
	languages[info.LangName] = info
}

func GetLanguage(name string) *LanguageInfo {
	name = strings.ToLower(strings.TrimSpace(name))
	if l, ok := languages[name]; ok {
		return l
	}
	return nil
}

func AllLanguages() map[string]*LanguageInfo {
	return languages
}

func HealthCheck() []string {
	var warnings []string
	for _, info := range languages {
		if info.Compiler != "" && info.Compiled {
			if _, err := exec.LookPath(info.compilerPath); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s compiler not found at %s", info.LangDisplayName, info.compilerPath))
			}
		}
		if info.Debugger != "" {
			if _, err := exec.LookPath(info.debuggerPath); err != nil {
				warnings = append(warnings, fmt.Sprintf("%s debugger not found at %s", info.LangDisplayName, info.debuggerPath))
			}
		}
	}
	return warnings
}
