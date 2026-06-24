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
	"unsafe"

	"golang.org/x/sys/unix"

	"ztutor/internal/logutil"
)

const (
	maxRuntime        = 5 * time.Second
	maxCompileRuntime = 30 * time.Second
	maxMemory         = 128 * 1024 * 1024 // 128 MB
	maxFileSize       = 8 * 1024 * 1024   // 8 MB
	maxOpenFiles      = 64
	maxProcs          = 8
	maxCPUSeconds     = 10 // RLIMIT_CPU in seconds
)

var canUseNamespaces bool

func init() {
	canUseNamespaces = probeNamespaces()
}

func probeNamespaces() bool {
	if os.Getenv("ZTUTOR_NO_NAMESPACES") != "" {
		return false
	}
	dir, err := os.MkdirTemp("", "ztutor-probe-")
	if err != nil {
		return false
	}
	defer os.RemoveAll(dir)

	progPath := filepath.Join(dir, "probe")
	os.WriteFile(progPath, nil, 0755)

	cmd := exec.Command(progPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: unix.CLONE_NEWUSER | unix.CLONE_NEWNS | unix.CLONE_NEWPID,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}
	err = cmd.Run()
	return err == nil
}

// rlimitEntry ties a resource identifier to its old-value holder.
type rlimitEntry struct {
	resource int
	cur      uint64
	max      uint64
	old      *syscall.Rlimit
}

func setResourceLimits() []rlimitEntry {
	var (
		oldMem    syscall.Rlimit
		oldFsize  syscall.Rlimit
		oldNofile syscall.Rlimit
		oldNproc  syscall.Rlimit
		oldCPU    syscall.Rlimit
		oldCore   syscall.Rlimit
	)

	limits := []rlimitEntry{
		{syscall.RLIMIT_AS, maxMemory, maxMemory, &oldMem},
		{syscall.RLIMIT_FSIZE, maxFileSize, maxFileSize, &oldFsize},
		{syscall.RLIMIT_NOFILE, maxOpenFiles, maxOpenFiles, &oldNofile},
		{unix.RLIMIT_NPROC, maxProcs, maxProcs, &oldNproc},
		{syscall.RLIMIT_CPU, maxCPUSeconds, maxCPUSeconds, &oldCPU},
		{syscall.RLIMIT_CORE, 0, 0, &oldCore},
	}

	for _, l := range limits {
		syscall.Getrlimit(l.resource, l.old)
		if err := syscall.Setrlimit(l.resource, &syscall.Rlimit{Cur: l.cur, Max: l.max}); err != nil {
			logutil.Warn("sandbox: Setrlimit(%d) failed: %v", l.resource, err)
		}
	}

	return limits
}

func restoreResourceLimits(limits []rlimitEntry) {
	for _, l := range limits {
		syscall.Setrlimit(l.resource, l.old) //nolint:errcheck
	}
}

func executionSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Pdeathsig: syscall.SIGKILL,
		Setpgid:   true,
	}
}

func setNamespaceOpts(cmd *exec.Cmd) {
	cmd.SysProcAttr.Cloneflags = unix.CLONE_NEWUSER | unix.CLONE_NEWNS | unix.CLONE_NEWNET | unix.CLONE_NEWPID
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: os.Getuid(), Size: 1},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: os.Getgid(), Size: 1},
	}
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

	if ctxErr != nil {
		return &Result{Output: output, Error: fmt.Sprintf("program timed out (%s)", maxRuntime)}
	}

	if execErr != nil {
		if exitErr, ok := execErr.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				sig := status.Signal()
				return &Result{Output: output, ExitCode: 128 + int(sig), Error: fmt.Sprintf("program crashed: %s", sig)}
			}
			return &Result{Output: output, ExitCode: exitErr.ExitCode()}
		}
		return &Result{Output: output, Error: execErr.Error()}
	}

	return &Result{Output: output, ExitCode: 0}
}

// ── Public types ──────────────────────────────────────────────────────────────

type Result struct {
	Output   string
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
	Stdin    string
	Args     []string
	Expected string
}

type TestResult struct {
	Num      int
	Passed   bool
	Got      string
	Want     string
	Error    string
	ExitCode int
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
		got := strings.TrimSpace(r.Output)
		want := strings.TrimSpace(tc.Expected)
		results[i] = TestResult{
			Num:      i + 1,
			Passed:   r.Error == "" && r.ExitCode == 0 && got == want,
			Got:      r.Output,
			Want:     tc.Expected,
			Error:    r.Error,
			ExitCode: r.ExitCode,
		}
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
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}
	// Ensure master is closed on early returns.
	closeMaster := true
	defer func() {
		if closeMaster {
			master.Close()
		}
	}()

	var ptyNum uint32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCGPTN, uintptr(unsafe.Pointer(&ptyNum))); errno != 0 {
		return nil, nil, nil, fmt.Errorf("TIOCGPTN: %w", errno)
	}
	slaveName := fmt.Sprintf("/dev/pts/%d", ptyNum)

	var lock int32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCSPTLCK, uintptr(unsafe.Pointer(&lock))); errno != 0 {
		return nil, nil, nil, fmt.Errorf("TIOCSPTLCK: %w", errno)
	}

	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open slave: %w", err)
	}
	defer slave.Close()

	if t, err2 := unix.IoctlGetTermios(int(slave.Fd()), unix.TCGETS); err2 == nil {
		t.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
		if err3 := unix.IoctlSetTermios(int(slave.Fd()), unix.TCSETS, t); err3 != nil {
			logutil.Warn("sandbox: interactive: failed to disable echo on pty: %v", err3)
		}
	}

	cmd := exec.Command(command, args...)
	cmd.Stdin = slave
	cmd.Stdout = slave
	cmd.Stderr = slave
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:    true,
		Setctty:   true,
		Ctty:      0,
		Pdeathsig: syscall.SIGKILL,
	}
	// Scrub environment: interactive mode gets the same minimal environment.
	dir, _ := os.MkdirTemp("", "ztutor-interactive-")
	if dir != "" {
		defer os.RemoveAll(dir)
	}
	cmd.Env = sandboxEnv(dir, nil)

	// Apply namespace isolation when available.
	if canUseNamespaces {
		cmd.SysProcAttr.Cloneflags = unix.CLONE_NEWUSER | unix.CLONE_NEWNS | unix.CLONE_NEWNET | unix.CLONE_NEWPID
		cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		}
		cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		}
		limits := setResourceLimits()
		defer restoreResourceLimits(limits)
	}

	if err := cmd.Start(); err != nil {
		return nil, nil, nil, fmt.Errorf("start: %w", err)
	}
	closeMaster = false // master will be closed by the goroutine

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
