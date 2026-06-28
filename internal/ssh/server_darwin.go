//go:build darwin

package ssh

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
)

func openPTY() (*os.File, string, error) {
	master, err := os.OpenFile("/dev/ptmx", os.O_RDWR|syscall.O_NOCTTY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open /dev/ptmx: %w", err)
	}

	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGRANT, 0); errno != 0 {
		master.Close()
		return nil, "", fmt.Errorf("TIOCPTYGRANT: %w", errno)
	}
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYUNLK, 0); errno != 0 {
		master.Close()
		return nil, "", fmt.Errorf("TIOCPTYUNLK: %w", errno)
	}

	var nameBuf [128]byte
	if _, _, errno := unix.Syscall(unix.SYS_IOCTL, master.Fd(), unix.TIOCPTYGNAME, uintptr(unsafe.Pointer(&nameBuf[0]))); errno != 0 {
		master.Close()
		return nil, "", fmt.Errorf("TIOCPTYGNAME: %w", errno)
	}
	n := 0
	for n < len(nameBuf) && nameBuf[n] != 0 {
		n++
	}
	slaveName := string(nameBuf[:n])

	return master, slaveName, nil
}
