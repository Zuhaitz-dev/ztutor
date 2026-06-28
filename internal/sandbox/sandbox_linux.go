//go:build linux

package sandbox

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"

	"ztutor/internal/logutil"
)

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
	os.WriteFile(progPath, nil, 0755) //nolint:errcheck

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
	return cmd.Run() == nil
}

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

func openInteractivePTY() (*os.File, *os.File, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, nil, fmt.Errorf("open /dev/ptmx: %w", err)
	}

	var ptyNum uint32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCGPTN, uintptr(unsafe.Pointer(&ptyNum))); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("TIOCGPTN: %w", errno)
	}
	slaveName := fmt.Sprintf("/dev/pts/%d", ptyNum)

	var lock int32
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCSPTLCK, uintptr(unsafe.Pointer(&lock))); errno != 0 {
		master.Close()
		return nil, nil, fmt.Errorf("TIOCSPTLCK: %w", errno)
	}

	slave, err := os.OpenFile(slaveName, os.O_RDWR|syscall.O_NOCTTY, 0)
	if err != nil {
		master.Close()
		return nil, nil, fmt.Errorf("open slave pty: %w", err)
	}

	return master, slave, nil
}

func configureTermios(slaveFd int) {
	t, err := unix.IoctlGetTermios(slaveFd, unix.TCGETS)
	if err != nil {
		return
	}
	t.Lflag &^= unix.ECHO | unix.ECHOE | unix.ECHOK | unix.ECHONL
	if err := unix.IoctlSetTermios(slaveFd, unix.TCSETS, t); err != nil {
		logutil.Warn("sandbox: interactive: failed to disable echo on pty: %v", err)
	}
}

func interactiveSysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{
		Setsid:    true,
		Setctty:   true,
		Ctty:      0,
		Pdeathsig: syscall.SIGKILL,
	}
}

func applyInteractiveIsolation(cmd *exec.Cmd) func() {
	if !canUseNamespaces {
		return func() {}
	}
	cmd.SysProcAttr.Cloneflags = unix.CLONE_NEWUSER | unix.CLONE_NEWNS | unix.CLONE_NEWNET | unix.CLONE_NEWPID
	cmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: os.Getuid(), Size: 1},
	}
	cmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
		{ContainerID: 0, HostID: os.Getgid(), Size: 1},
	}
	limits := setResourceLimits()
	return func() { restoreResourceLimits(limits) }
}
