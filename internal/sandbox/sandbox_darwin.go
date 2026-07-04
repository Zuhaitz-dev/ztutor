//go:build darwin

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	"ztutor/internal/logutil"
)

func probeNamespaces() bool {
	return false
}

type rlimitEntry struct {
	resource int
	cur      uint64
	max      uint64
	old      *syscall.Rlimit
}

func setResourceLimits() []rlimitEntry {
	var (
		oldFsize  syscall.Rlimit
		oldNofile syscall.Rlimit
		oldNproc  syscall.Rlimit
		oldCPU    syscall.Rlimit
		oldCore   syscall.Rlimit
	)

	limits := []rlimitEntry{
		{syscall.RLIMIT_FSIZE, Limits.MaxFileSize, Limits.MaxFileSize, &oldFsize},
		{syscall.RLIMIT_NOFILE, Limits.MaxOpenFiles, Limits.MaxOpenFiles, &oldNofile},
		{unix.RLIMIT_NPROC, Limits.MaxProcs, Limits.MaxProcs, &oldNproc},
		{syscall.RLIMIT_CPU, Limits.MaxCPUSeconds, Limits.MaxCPUSeconds, &oldCPU},
		{syscall.RLIMIT_CORE, 0, 0, &oldCore},
	}

	for _, l := range limits {
		syscall.Getrlimit(l.resource, l.old) //nolint:errcheck
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
	return &syscall.SysProcAttr{Setpgid: true}
}

func setNamespaceOpts(_ *exec.Cmd) {}

func openInteractivePTY() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGRANT, 0); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("TIOCPTYGRANT: %w", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYUNLK, 0); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("TIOCPTYUNLK: %w", errno)
	}

	var nameBuf [128]byte
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&nameBuf[0]))); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("TIOCPTYGNAME: %w", errno)
	}
	n := 0
	for n < len(nameBuf) && nameBuf[n] != 0 {
		n++
	}
	slaveName := string(nameBuf[:n])

	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("open slave pty %s: %w", slaveName, err)
	}
	return master, slave, nil
}

func configureTermios(slaveFd int) {
	t, err := unix.IoctlGetTermios(slaveFd, unix.TIOCGETA)
	if err != nil {
		return
	}
	t.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	if err := unix.IoctlSetTermios(slaveFd, unix.TIOCSETA, t); err != nil {
		logutil.Warn("sandbox: interactive: failed to disable echo on pty: %v", err)
	}
}

func interactiveSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid:  true,
		Setctty: true,
		Ctty:    0,
	}
}

func applyInteractiveIsolation(_ *exec.Cmd) func() {
	return func() {}
}
