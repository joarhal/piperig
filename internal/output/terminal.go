package output

import (
	"os"
	"syscall"
	"unsafe"
)

// IsTerminal reports whether fd refers to a terminal.
func IsTerminal(fd uintptr) bool {
	var wsz [8]byte // sizeof(struct winsize) = 8
	_, _, err := syscall.Syscall(syscall.SYS_IOCTL, fd, syscall.TIOCGWINSZ, uintptr(unsafe.Pointer(&wsz[0])))
	return err == 0
}

// StdoutIsTerminal reports whether stdout is a terminal.
func StdoutIsTerminal() bool {
	return IsTerminal(os.Stdout.Fd())
}
