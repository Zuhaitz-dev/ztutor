//go:build windows

package sandbox

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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

func sandboxPathEnv() string { return `C:\Windows\system32;C:\Windows` }

// exitSignalInfo is a no-op on Windows: crashes surface as exit codes, not
// signals, so the 128+sig path never applies.
func exitSignalInfo(execErr error) (int, bool) {
	return 0, false
}

// spawnPTYChild starts `command` attached to a ConPTY pseudo-terminal. When
// isolated is true (interactive mode) the child gets a sandboxed environment;
// the debugger runs without isolation.
func spawnPTYChild(command string, args []string, isolated bool) (*ptyChild, error) {
	// Console size in character cells. The TUI resizes do not currently drive
	// ResizePseudoConsole; a fixed reasonable size keeps REPLs happy.
	size := windows.Coord{X: 120, Y: 30}

	// The pseudo console bridges an input pipe (parent writes inW) and an
	// output pipe (parent reads outR).
	inR, inW, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	outR, outW, err := os.Pipe()
	if err != nil {
		inR.Close()
		inW.Close()
		return nil, err
	}

	var console windows.Handle
	if err := windows.CreatePseudoConsole(size, windows.Handle(inR.Fd()), windows.Handle(outW.Fd()), 0, &console); err != nil {
		inR.Close()
		inW.Close()
		outR.Close()
		outW.Close()
		return nil, fmt.Errorf("CreatePseudoConsole: %w", err)
	}
	// The parent only needs inW and outR from here on.
	inR.Close()
	outW.Close()

	cmdLine, err := buildCommandLine(command, args)
	if err != nil {
		inW.Close()
		outR.Close()
		windows.ClosePseudoConsole(console)
		return nil, err
	}

	attrList, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		inW.Close()
		outR.Close()
		windows.ClosePseudoConsole(console)
		return nil, err
	}
	if err := attrList.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(&console), unsafe.Sizeof(console)); err != nil {
		attrList.Delete()
		inW.Close()
		outR.Close()
		windows.ClosePseudoConsole(console)
		return nil, err
	}

	startupInfo := windows.StartupInfoEx{
		StartupInfo:             windows.StartupInfo{Cb: uint32(unsafe.Sizeof(windows.StartupInfoEx{}))},
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
			inW.Close()
			outR.Close()
			windows.ClosePseudoConsole(console)
			return nil, err
		}
		flags |= windows.CREATE_UNICODE_ENVIRONMENT
	}

	cmdLinePtr, err := windows.UTF16PtrFromString(cmdLine)
	if err != nil {
		attrList.Delete()
		inW.Close()
		outR.Close()
		windows.ClosePseudoConsole(console)
		return nil, err
	}

	var pi windows.ProcessInformation
	if err := windows.CreateProcess(nil, cmdLinePtr, nil, nil, false, flags, envBlock, nil, &startupInfo.StartupInfo, &pi); err != nil {
		attrList.Delete()
		inW.Close()
		outR.Close()
		windows.ClosePseudoConsole(console)
		return nil, fmt.Errorf("CreateProcess: %w", err)
	}
	attrList.Delete()

	var closeOnce sync.Once
	cleanup := func() {
		closeOnce.Do(func() {
			windows.CloseHandle(pi.Thread)
			windows.CloseHandle(pi.Process)
			windows.ClosePseudoConsole(console)
			inW.Close()
			outR.Close()
		})
	}

	return &ptyChild{
		read:  func(b []byte) (int, error) { return outR.Read(b) },
		write: func(b []byte) (int, error) { return inW.Write(b) },
		kill: func() {
			_ = windows.TerminateProcess(pi.Process, 1)
			cleanup()
		},
		wait: func() int {
			_, _ = windows.WaitForSingleObject(pi.Process, windows.INFINITE)
			var code uint32
			_ = windows.GetExitCodeProcess(pi.Process, &code)
			cleanup()
			return int(code)
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

// staticLinkFlags statically links MinGW C/C++ binaries so they don't depend
// on the MinGW runtime DLLs (libgcc_s, libstdc++, winpthread) at run time —
// the sandboxed environment's curated PATH does not include the compiler dir.
func staticLinkFlags() []string { return []string{"-static"} }

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
