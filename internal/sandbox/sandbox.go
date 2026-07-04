package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"
)

var Limits = SandboxLimits{
	MaxRuntime:        5 * time.Second,
	MaxCompileRuntime: 30 * time.Second,
	MaxMemory:         128 * 1024 * 1024, // 128 MB
	MaxFileSize:       8 * 1024 * 1024,   // 8 MB
	MaxOpenFiles:      64,
	MaxProcs:          8,
	MaxCPUSeconds:     10,
}

type SandboxLimits struct {
	MaxRuntime        time.Duration
	MaxCompileRuntime time.Duration
	MaxMemory         int64
	MaxFileSize       uint64
	MaxOpenFiles      uint64
	MaxProcs          uint64
	MaxCPUSeconds     uint64
}

var canUseNamespaces bool

func init() {
	canUseNamespaces = probeNamespaces()
}

func formatExecuteResult(stdout, stderr, dir string, ctxErr error, execErr error) *Result {
	output := stdout
	if stderr != "" {
		if output != "" {
			output += "\n"
		}
		output += stderr
	}
	output = stripDir(output, dir)
	stdout = stripDir(stdout, dir)
	stderr = stripDir(stderr, dir)

	if ctxErr != nil {
		return &Result{Output: output, Stdout: stdout, Stderr: stderr, Error: fmt.Sprintf("program timed out (%s)", Limits.MaxRuntime)}
	}

	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				sig := status.Signal()
				return &Result{Output: output, Stdout: stdout, Stderr: stderr, ExitCode: 128 + int(sig), Error: fmt.Sprintf("program crashed: %s", sig)}
			}
			return &Result{Output: output, Stdout: stdout, Stderr: stderr, ExitCode: exitErr.ExitCode()}
		}
		return &Result{Output: output, Stdout: stdout, Stderr: stderr, Error: execErr.Error()}
	}

	return &Result{Output: output, Stdout: stdout, Stderr: stderr, ExitCode: 0}
}

// ── Public types ──────────────────────────────────────────────────────────────

type Result struct {
	Output   string
	Stdout   string
	Stderr   string
	ExitCode int
	Error    string
}

type DebugBuild struct {
	BinaryPath  string
	RuntimeArgs []string
	dir         string
}

func (d *DebugBuild) Close() {
	if d != nil {
		os.RemoveAll(d.dir)
	}
}

type TestInput struct {
	Stdin             string
	Args              []string
	Expected          string
	ExpectedStdout    string
	ExpectedStderr    string
	HasExpectedStdout bool
	HasExpectedStderr bool
}

type TestResult struct {
	Num      int
	Passed   bool
	Got      string
	Want     string
	Error    string
	ExitCode int
}

func formatExpectedStreams(stdout string, hasStdout bool, stderr string, hasStderr bool) string {
	var parts []string
	if hasStdout {
		parts = append(parts, "[stdout]\n"+stdout)
	}
	if hasStderr {
		parts = append(parts, "[stderr]\n"+stderr)
	}
	return strings.Join(parts, "\n")
}

func CompareResult(num int, r *Result, tc TestInput) TestResult {
	if r == nil {
		return TestResult{Num: num, Error: "missing execution result"}
	}

	got := r.Output
	want := tc.Expected
	passed := strings.TrimSpace(r.Output) == strings.TrimSpace(tc.Expected)

	if tc.HasExpectedStdout || tc.HasExpectedStderr {
		got = formatExpectedStreams(r.Stdout, tc.HasExpectedStdout, r.Stderr, tc.HasExpectedStderr)
		want = formatExpectedStreams(tc.ExpectedStdout, tc.HasExpectedStdout, tc.ExpectedStderr, tc.HasExpectedStderr)
		passed = true
		if tc.HasExpectedStdout && strings.TrimSpace(r.Stdout) != strings.TrimSpace(tc.ExpectedStdout) {
			passed = false
		}
		if tc.HasExpectedStderr && strings.TrimSpace(r.Stderr) != strings.TrimSpace(tc.ExpectedStderr) {
			passed = false
		}
	}

	return TestResult{
		Num:      num,
		Passed:   r.Error == "" && r.ExitCode == 0 && passed,
		Got:      got,
		Want:     want,
		Error:    r.Error,
		ExitCode: r.ExitCode,
	}
}

// ── Multi-file helpers ────────────────────────────────────────────────────────

// safeWritePath validates that name doesn't escape dir via path traversal and
// returns the cleaned absolute path within dir.
func safeWritePath(dir, name string) (string, error) {
	if name == "" || strings.Contains(name, "..") || filepath.IsAbs(name) {
		return "", fmt.Errorf("invalid filename: %q", name)
	}
	p := filepath.Join(dir, name)
	cleanDir := filepath.Clean(dir) + string(os.PathSeparator)
	if !strings.HasPrefix(filepath.Clean(p), cleanDir) {
		return "", fmt.Errorf("path traversal rejected: %q", name)
	}
	return p, nil
}

// writeFiles writes the contents of files to dir, creating subdirectories as needed.
// Rejects filenames containing "..", absolute paths, or paths that escape dir.
func writeFiles(dir string, files map[string]string) error {
	for name, content := range files {
		p, err := safeWritePath(dir, name)
		if err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
		if sub := filepath.Dir(p); sub != dir && strings.HasPrefix(sub, filepath.Clean(dir)+string(os.PathSeparator)) {
			if err := os.MkdirAll(sub, 0755); err != nil {
				return fmt.Errorf("mkdir %s: %w", sub, err)
			}
		}
		if err := os.WriteFile(p, []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}
	return nil
}

// ensureProg makes sure dir/prog exists after a custom build command.
// If the build produced a differently-named binary (e.g. "redis-server"),
// it renames it to "prog" so the subsequent lang.Execute() call can find it.
func ensureProg(dir string) {
	progPath := filepath.Join(dir, "prog")
	if _, err := os.Stat(progPath); err == nil {
		return
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() && info.Mode()&0111 != 0 {
			os.Rename(filepath.Join(dir, e.Name()), progPath)
			return
		}
	}
}

// runBuildCmd executes a build command (e.g. "make") in dir.
// Non-empty Result.Error means the build failed.
func runBuildCmd(dir, buildCmd string) *Result {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	parts := strings.Fields(buildCmd)
	if len(parts) == 0 {
		return &Result{Error: "empty build command"}
	}
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = dir
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return &Result{Error: fmt.Sprintf("build timed out (%s)", 30*time.Second)}
		}
		out := buf.String()
		if out == "" {
			out = err.Error()
		}
		return &Result{Error: out}
	}
	return &Result{Output: buf.String()}
}

// primarySrcPaths returns the absolute paths of source files matching lang's extension.
// Falls back to the canonical source file name when no files match.
func primarySrcPaths(dir string, lang Language, files map[string]string) []string {
	ext := lang.SourceExtension()
	var paths []string
	for name := range files {
		if strings.HasSuffix(strings.ToLower(name), ext) {
			paths = append(paths, filepath.Join(dir, name))
		}
	}
	sort.Strings(paths)
	if len(paths) == 0 {
		return []string{filepath.Join(dir, lang.SourceFileName())}
	}
	return paths
}

// ── Language-aware public API ─────────────────────────────────────────────────

// Run compiles (or runs) the given files and returns the execution result.
// files is a map of filename → content; buildCmd overrides language compilation when set.
// When buildCmd is used, it must produce a binary at dir/prog which lang.Execute then runs.
func Run(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	dir, err := os.MkdirTemp("", "ztutor-sandbox-")
	if err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := writeFiles(dir, files); err != nil {
		return nil, fmt.Errorf("write files: %w", err)
	}

	if buildCmd != "" {
		if r := runBuildCmd(dir, buildCmd); r.Error != "" {
			return r, nil
		}
		ensureProg(dir)
	} else if lang.IsCompiled() {
		srcPaths := primarySrcPaths(dir, lang, files)
		flags := appendSlice(lang.DefaultFlags(), extraFlags)
		if r := lang.CompileFiles(dir, srcPaths, flags); r.Error != "" {
			return r, nil
		}
	} else {
		srcPaths := primarySrcPaths(dir, lang, files)
		flags := appendSlice(lang.DefaultFlags(), extraFlags)
		if r := lang.CheckSyntax(dir, srcPaths[0], flags); r != nil && r.Error != "" {
			return r, nil
		}
	}

	return lang.Execute(dir, stdin, runtimeArgs, nil)
}

// RunWithASAN compiles files with ASAN/UBSan enabled and runs the result.
func RunWithASAN(lang Language, files map[string]string, buildCmd, stdin string, extraFlags, runtimeArgs []string) (*Result, error) {
	if !lang.HasSanitizers() {
		return &Result{Error: "sanitizers not supported for " + lang.Name()}, nil
	}
	dir, err := os.MkdirTemp("", "ztutor-sandbox-")
	if err != nil {
		return nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := writeFiles(dir, files); err != nil {
		return nil, fmt.Errorf("write files: %w", err)
	}

	if buildCmd != "" {
		if r := runBuildCmd(dir, buildCmd); r.Error != "" {
			return r, nil
		}
		ensureProg(dir)
	} else {
		srcPaths := primarySrcPaths(dir, lang, files)
		flags := appendSlice(lang.SanitizerFlags(), extraFlags)
		if r := lang.CompileFiles(dir, srcPaths, flags); r.Error != "" {
			return r, nil
		}
	}

	asanEnv := []string{
		"ASAN_OPTIONS=detect_leaks=0:color=always:print_summary=1",
		"UBSAN_OPTIONS=print_stacktrace=1:color=always",
	}
	return lang.Execute(dir, stdin, runtimeArgs, asanEnv)
}

// CompileDebug compiles files with debug symbols for GDB / interactive mode.
func CompileDebug(lang Language, files map[string]string, buildCmd string, extraFlags []string) (*DebugBuild, *Result) {
	dir, err := os.MkdirTemp("", "ztutor-sandbox-")
	if err != nil {
		return nil, &Result{Error: fmt.Sprintf("create sandbox dir: %v", err)}
	}

	if err := writeFiles(dir, files); err != nil {
		os.RemoveAll(dir)
		return nil, &Result{Error: fmt.Sprintf("write files: %v", err)}
	}

	if buildCmd != "" {
		if r := runBuildCmd(dir, buildCmd); r.Error != "" {
			os.RemoveAll(dir)
			return nil, r
		}
		ensureProg(dir)
		return &DebugBuild{BinaryPath: filepath.Join(dir, "prog"), dir: dir}, &Result{}
	}

	srcPaths := primarySrcPaths(dir, lang, files)
	flags := appendSlice(lang.DebugFlags(), extraFlags)
	if len(srcPaths) == 1 {
		dbg, r := lang.CompileDebug(dir, srcPaths[0], flags)
		if r != nil && r.Error != "" {
			os.RemoveAll(dir)
			return nil, r
		}
		return dbg, r
	}
	if r := lang.CompileFiles(dir, srcPaths, flags); r.Error != "" {
		os.RemoveAll(dir)
		return nil, r
	}
	return &DebugBuild{BinaryPath: filepath.Join(dir, "prog"), dir: dir}, &Result{}
}

// GenerateAssembly produces the assembly listing for the primary source file.
func GenerateAssembly(lang Language, files map[string]string, buildCmd string, extraFlags []string) (string, error) {
	dir, err := os.MkdirTemp("", "ztutor-sandbox-")
	if err != nil {
		return "", fmt.Errorf("create sandbox dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := writeFiles(dir, files); err != nil {
		return "", fmt.Errorf("write files: %w", err)
	}

	srcPaths := primarySrcPaths(dir, lang, files)
	flags := appendSlice(lang.AsmFlags(), extraFlags)
	return lang.GenerateAssembly(dir, srcPaths[0], flags)
}

// SyntaxCheck runs a syntax-only compile on the primary source file and returns diagnostics.
func SyntaxCheck(lang Language, files map[string]string, buildCmd string, extraFlags []string) ([]Diagnostic, error) {
	dir, err := os.MkdirTemp("", "ztutor-diag-")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := writeFiles(dir, files); err != nil {
		return nil, fmt.Errorf("write files: %w", err)
	}

	srcPaths := primarySrcPaths(dir, lang, files)
	flags := appendSlice(lang.DefaultFlags(), extraFlags)
	result := lang.CheckSyntax(dir, srcPaths[0], flags)
	return lang.ParseDiagnostics(result.Output), nil
}

// RunAllTests compiles files once then runs each test case against the binary.
func RunAllTests(lang Language, files map[string]string, buildCmd string, extraFlags []string, tests []TestInput) (*Result, []TestResult, error) {
	dir, err := os.MkdirTemp("", "ztutor-sandbox-")
	if err != nil {
		return nil, nil, fmt.Errorf("create sandbox dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := writeFiles(dir, files); err != nil {
		return nil, nil, fmt.Errorf("write files: %w", err)
	}

	var compileRes *Result
	if buildCmd != "" {
		compileRes = runBuildCmd(dir, buildCmd)
		if compileRes.Error != "" {
			return compileRes, nil, nil
		}
		ensureProg(dir)
	} else if lang.IsCompiled() {
		srcPaths := primarySrcPaths(dir, lang, files)
		flags := appendSlice(lang.DefaultFlags(), extraFlags)
		compileRes = lang.CompileFiles(dir, srcPaths, flags)
		if compileRes.Error != "" {
			return compileRes, nil, nil
		}
	} else {
		compileRes = &Result{}
	}

	results := make([]TestResult, len(tests))
	for i, tc := range tests {
		r, execErr := lang.Execute(dir, tc.Stdin, tc.Args, nil)
		if execErr != nil {
			results[i] = TestResult{Num: i + 1, Error: execErr.Error()}
			continue
		}
		results[i] = CompareResult(i+1, r, tc)
	}
	return compileRes, results, nil
}

// ── PTY-based interactive execution ───────────────────────────────────────────

type InteractiveEvent struct {
	Text string
	Done bool
	Code int
}

func RunInteractive(command string, args []string) (writeFn func([]byte) error, events <-chan InteractiveEvent, kill func(), err error) {
	master, slave, err := openInteractivePTY()
	if err != nil {
		return nil, nil, nil, err
	}
	defer slave.Close()

	configureTermios(int(slave.Fd()))

	cmd := exec.Command(command, args...)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = interactiveSysProcAttr()

	dir, _ := os.MkdirTemp("", "ztutor-interactive-")
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	cmd.Env = sandboxEnv(dir, nil)

	cleanup := applyInteractiveIsolation(cmd)
	defer cleanup()

	closeMaster := true
	defer func() {
		if closeMaster {
			master.Close()
		}
	}()

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start: %w", err)
	}
	closeMaster = false

	ch := make(chan InteractiveEvent, 64)

	go func() {
		defer master.Close()
		defer close(ch)

		buf := make([]byte, 512)
		for {
			n, err := master.Read(buf)
			if n > 0 {
				text := strings.ReplaceAll(string(buf[:n]), "\r\n", "\n")
				text = strings.ReplaceAll(text, "\r", "")
				ch <- InteractiveEvent{Text: text}
			}
			if err != nil {
				break
			}
		}

		exitCode := 0
		if err := cmd.Wait(); err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			}
		}
		ch <- InteractiveEvent{Done: true, Code: exitCode}
	}()

	writeFn = func(data []byte) error {
		_, err := master.Write(data)
		return err
	}
	kill = func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}
	return writeFn, ch, kill, nil
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// sandboxEnv returns a minimal environment for the sandboxed child process.
// Only safe variables (PATH, HOME set to the sandbox dir, TMPDIR, LANG) are
// passed; the parent's secrets, AWS keys, XDG variables, etc. are scrubbed.
func sandboxEnv(dir string, extraEnv []string) []string {
	env := []string{
		"PATH=" + os.Getenv("PATH"),
		"HOME=" + dir,
		"TMPDIR=" + dir,
		"TEMP=" + dir,
		"TMP=" + dir,
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
	}
	if len(extraEnv) > 0 {
		env = append(env, extraEnv...)
	}
	return env
}

func stripDir(s, dir string) string {
	return strings.ReplaceAll(s, dir+"/", "")
}

func appendSlice(a, b []string) []string {
	if len(b) == 0 {
		if len(a) == 0 {
			return nil
		}
		out := make([]string, len(a))
		copy(out, a)
		return out
	}
	out := make([]string, len(a)+len(b))
	copy(out, a)
	copy(out[len(a):], b)
	return out
}
