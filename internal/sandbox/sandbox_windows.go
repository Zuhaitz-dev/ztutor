//go:build windows

package sandbox

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

func probeNamespaces() bool {
	return false
}

// rlimitEntry is unused on Windows; resource limits are not enforced the same
// way (Job Objects would be required).
type rlimitEntry struct{}

func setResourceLimits() []rlimitEntry { return nil }

func restoreResourceLimits(_ []rlimitEntry) {}

// executionSysProcAttr returns process attributes for sandboxed children.
// The child is created without a console window (this is a server process).
func executionSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func setNamespaceOpts(_ *exec.Cmd) {}

func makeExecutable(path string) error { return nil }

func sandboxBinaryName() string { return "prog.exe" }

func sandboxPathEnv() string {
	dirs := []string{`C:\Windows\system32`, `C:\Windows`}
	// Include the compiler directories so MinGW binaries can load their
	// runtime DLLs (libgcc_s, libstdc++, winpthread) at execution time.
	dirs = append(dirs, sandboxToolchainDirs...)
	return strings.Join(dirs, ";")
}

// exitSignalInfo is a no-op on Windows: crashes surface as exit codes, not
// signals, so the 128+sig path never applies.
func exitSignalInfo(execErr error) (int, bool) {
	return 0, false
}

// spawnPTYChild starts `command` attached to a ConPTY pseudo-terminal. When
// isolated is true (interactive mode) the child gets a sandboxed environment;
// the debugger runs without isolation.
func closeConPtyHandles(handles ...windows.Handle) {
	for _, h := range handles {
		if h != windows.InvalidHandle {
			_ = windows.CloseHandle(h)
		}
	}
}

func spawnPTYChild(command string, args []string, isolated bool) (*ptyChild, error) {
	// Console size in character cells. The TUI resizes do not currently drive
	// ResizePseudoConsole; a fixed reasonable size keeps REPLs happy.
	size := windows.Coord{X: 120, Y: 30}

	// The pseudo console bridges an input pipe (parent writes inW) and an
	// output pipe (parent reads outR). Raw handles + blocking ReadFile/WriteFile
	// (as the conpty library does) behave better with ConPTY teardown than
	// os.File's poller-driven I/O.
	var inR, inW, outR, outW windows.Handle
	if err := windows.CreatePipe(&inR, &inW, nil, 0); err != nil {
		return nil, err
	}
	if err := windows.CreatePipe(&outR, &outW, nil, 0); err != nil {
		closeConPtyHandles(inR, inW)
		return nil, err
	}

	var console windows.Handle
	if err := windows.CreatePseudoConsole(size, inR, outW, 0, &console); err != nil {
		closeConPtyHandles(inR, inW, outR, outW)
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}
	// Keep inR/outW open: inR is the console's input read end, and outW must
	// stay open until the child exits so the reader can drain buffered output
	// before the pipe signals EOF.

	cmdLine, err := buildCommandLine(command, args)
	if err != nil {
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		windows.CloseHandle(outR)
		windows.CloseHandle(outW)
		windows.ClosePseudoConsole(console)
		return nil, err
	}

	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		windows.CloseHandle(outR)
		windows.CloseHandle(outW)
		windows.ClosePseudoConsole(console)
		return nil, err
	}
	// The PSEUDOCONSOLE attribute value is the HPCON handle itself, not a
	// pointer to it. Pass it as a plain uintptr argument to avoid the
	// unsafe.Pointer(uintptr(...)) conversion (which go vet rejects).
	updateAttr := windows.NewLazySystemDLL("kernel32.dll").NewProc("UpdateProcThreadAttribute")
	ret, _, e := updateAttr.Call(
		uintptr(unsafe.Pointer(attrList.List())),
		0,
		uintptr(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE),
		uintptr(console),
		unsafe.Sizeof(console),
		0,
		0)
	if ret == 0 {
		attrList.Delete()
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		windows.CloseHandle(outR)
		windows.CloseHandle(outW)
		windows.ClosePseudoConsole(console)
		return nil, e
	}

	startupInfo := windows.StartupInfoEx{
		StartupInfo: windows.StartupInfo{
			Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{})),
		},
		ProcThreadAttributeList: attrList.List(),
	}

	flags := uint32(windows.EXTENDED_STARTUPINFO_PRESENT)
	var envBlock *uint16
	if isolated {
		dir, _ := os.MkdirTemp("", "ztutor-interactive-")
		if dir != "" {
			defer os.RemoveAll(dir)
		}
		envBlock, err = buildEnvBlock(sandboxEnv(dir, nil))
		if err != nil {
			attrList.Delete()
			windows.CloseHandle(inR)
			windows.CloseHandle(inW)
			windows.CloseHandle(outR)
			windows.CloseHandle(outW)
			windows.ClosePseudoConsole(console)
			return nil, err
		}
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
	}

	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		attrList.Delete()
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		windows.CloseHandle(outR)
		windows.CloseHandle(outW)
		windows.ClosePseudoConsole(console)
		return nil, err
	}

	var pi windows.ProcessInformation
	if err := windows.CreateProcess(nil, cmdLinePtr, nil, nil, false, flags, envBlock, nil, &startupInfo.StartupInfo, &pi); err != nil {
		attrList.Delete()
		windows.CloseHandle(inR)
		windows.CloseHandle(inW)
		windows.CloseHandle(outR)
		windows.CloseHandle(outW)
		windows.ClosePseudoConsole(console)
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}
	attrList.Delete()
	conptyDebugf("child started pid=%d command=%q", pi.ProcessId, command)

	// Watch the child: when it exits, give the pseudo console time to flush
	// pending output to the pipe BEFORE closing the write end and the console.
	// Closing outW or the console while the console's forward thread is still
	// writing drops a fast-exiting program's final output.
	exitCodeCh := make(chan int, 1)
	go func() {
		_, _ = windows.WaitForSingleObject(pi.Process, windows.INFINITE)
		var code uint32
		_ = windows.GetExitCodeProcess(pi.Process, &code)
		conptyDebugf("wait exit code=%d", code)
		time.Sleep(2 * time.Second)
		windows.CloseHandle(outW)
		windows.ClosePseudoConsole(console)
		exitCodeCh <- int(code)
	}()

	var closeOnce sync.Once
	cleanup := func() {
		closeOnce.Do(func() {
			windows.CloseHandle(pi.Thread)
			windows.CloseHandle(pi.Process)
			windows.CloseHandle(inR)
			windows.CloseHandle(inW)
			windows.CloseHandle(outR)
			windows.CloseHandle(outW)
		})
	}

	return &ptyChild{
		read: func(b []byte) (int, error) {
			var n uint32
			err := windows.ReadFile(outR, b, &n, nil)
			if err == windows.ERROR_BROKEN_PIPE || err == windows.ERROR_NO_DATA || err == windows.ERROR_INVALID_HANDLE {
				return int(n), io.EOF
			}
			return int(n), err
		},
		// The console's cooked-mode line editor submits a line on Enter (\r);
		// translate a bare \n to \r\n so programs blocking on stdin see it.
		write: func(b []byte) (int, error) {
			if bytes.IndexByte(b, '\n') >= 0 {
				b = bytes.ReplaceAll(b, []byte("\n"), []byte("\r\n"))
			}
			var n uint32
			err := windows.WriteFile(inW, b, &n, nil)
			return int(n), err
		},
		kill: func() {
			// Terminate and close the output write end so the reader unblocks.
			// The exit goroutine delivers the code; wait() performs cleanup.
			_ = windows.TerminateProcess(pi.Process, 1)
			windows.CloseHandle(outW)
		},
		wait: func() int {
			code := <-exitCodeCh
			cleanup()
			return code
		},
	}, nil
}

// buildCommandLine quotes command and args for CreateProcess.
func buildCommandLine(command string, args []string) (string, error) {
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, quoteArg(command))
	for _, a := range args {
		parts = append(parts, quoteArg(a))
	}
	return strings.Join(parts, " "), nil
}

// quoteArg applies the Windows CreateProcess command-line quoting rules.
func quoteArg(s string) string {
	if s == "" {
		return `""`
	}
	needsQuoting := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '"' {
			needsQuoting = true
			break
		}
	}
	if !needsQuoting {
		return s
	}
	var b strings.Builder
	b.WriteByte('"')
	i := 0
	for i < len(s) {
		bs := 0
		for i < len(s) && s[i] == '\\' {
			i++
			bs++
		}
		if i == len(s) {
			b.WriteString(strings.Repeat(`\`, bs*2))
			break
		}
		if s[i] == '"' {
			b.WriteString(strings.Repeat(`\`, bs*2+1))
			b.WriteByte('"')
			i++
		} else {
			b.WriteString(strings.Repeat(`\`, bs))
			b.WriteByte(s[i])
			i++
		}
	}
	b.WriteByte('"')
	return b.String()
}

// buildEnvBlock encodes an environment slice as a UTF-16 environment block.
func buildEnvBlock(env []string) (*uint16, error) {
	var buf []uint16
	for _, kv := range env {
		if kv == "" {
			continue
		}
		for _, r := range kv {
			buf = append(buf, uint16(r))
		}
		buf = append(buf, 0)
	}
	buf = append(buf, 0)
	return &buf[0], nil
}

// staticLinkFlags is a no-op: MinGW runtime DLLs are found by extending the
// sandbox PATH with the toolchain directories (see sandboxPathEnv).
func staticLinkFlags() []string { return nil }

// isExecutableCandidate reports whether a build artifact is the runnable
// binary. Windows files never carry the unix exec bit, so match known
// executable extensions (MinGW gcc/make produce prog.exe).
func isExecutableCandidate(name string, info fs.FileInfo) bool {
	if !info.Mode().IsRegular() {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".exe", ".bat", ".cmd", ".com":
		return true
	}
	return false
}

// conptyDebugf logs to stderr for debugging the ConPTY path on Windows CI.
func conptyDebugf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "[conpty] "+format+"\n", args...)
}
