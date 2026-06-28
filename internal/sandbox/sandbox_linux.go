//go:build linux

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
	if os.Getenv("ZTUTOR_NO_NAMESPACES") != "" {
		return false
	}
	cmd := exec.Command("/bin/true")
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
		oldFsize  syscall.Rlimit
		oldNofile syscall.Rlimit
		oldCore   syscall.Rlimit
	)

	// RLIMIT_AS, RLIMIT_NPROC, and RLIMIT_CPU are intentionally omitted:
	// all three are process-wide limits enforced before fork, which means they
	// affect the parent Go process too. RLIMIT_AS starves thread stacks,
	// RLIMIT_NPROC blocks fork when the user has many processes, and RLIMIT_CPU
	// kills the parent test runner on multi-core machines where accumulated CPU
	// time reaches the limit quickly. Namespace isolation + the 5-second
	// context deadline already cover the same threat model safely.
	limits := []rlimitEntry{
		{syscall.RLIMIT_FSIZE, maxFileSize, maxFileSize, &oldFsize},
		{syscall.RLIMIT_NOFILE, maxOpenFiles, maxOpenFiles, &oldNofile},
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
