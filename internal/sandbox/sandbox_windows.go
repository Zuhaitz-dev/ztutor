//go:build windows

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
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

// openInteractivePTY returns the pseudo-terminal pair for interactive mode.
// Phase C replaces this stub with a ConPTY-backed implementation.
func openInteractivePTY() (*os.File, *os.File, error) {
	return nil, nil, fmt.Errorf("interactive mode is not available on Windows yet")
}

func configureTermios(_ int) {}

func interactiveSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
		HideWindow:    true,
	}
}

func debuggerSysProcAttr() *syscall.SysProcAttr { return interactiveSysProcAttr() }

func applyInteractiveIsolation(_ *exec.Cmd) func() {
	return func() {}
}

// exitSignalInfo is a no-op on Windows: crashes surface as exit codes, not
// signals, so the 128+sig path never applies.
func exitSignalInfo(execErr error) (int, bool) {
	return 0, false
}

func sandboxBinaryName() string { return "prog.exe" }

// makeExecutable is a no-op on Windows (no POSIX permission bits).
func makeExecutable(path string) error { return nil }

func sandboxPathEnv() string { return `C:\Windows\system32;C:\Windows` }
