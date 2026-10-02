//go:build linux

package tui

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	ioctlTCGETS     = 0x5401
	ioctlTCSETS     = 0x5402
	ioctlTIOCGWINSZ = 0x5413
)

// winsize mirrors the struct used by TIOCGWINSZ.
type winsize struct {
	Rows, Cols, XPixel, YPixel uint16
}

// rawMode saves and restores terminal attributes using raw syscalls so the
// project keeps its zero-dependency guarantee.
type rawMode struct {
	fd     int
	saved  syscall.Termios
	active bool
}

func ioctl(fd int, req uintptr, arg unsafe.Pointer) error {
	if _, _, errno := syscall.Syscall(syscall.SYS_IOCTL, uintptr(fd), req, uintptr(arg)); errno != 0 {
		return errno
	}
	return nil
}

// Enter switches the terminal into raw mode (no echo, no line buffering, no
// signal generation so Ctrl+C arrives as a byte).
func (r *rawMode) Enter() error {
	var t syscall.Termios
	if err := ioctl(r.fd, ioctlTCGETS, unsafe.Pointer(&t)); err != nil {
		return err
	}
	r.saved = t
	mod := t
	mod.Lflag &^= syscall.ECHO | syscall.ICANON | syscall.IEXTEN | syscall.ISIG | syscall.BRKINT
	mod.Iflag &^= syscall.ICRNL | syscall.INLCR | syscall.IGNCR
	mod.Cc[syscall.VMIN] = 1
	mod.Cc[syscall.VTIME] = 0
	if err := ioctl(r.fd, ioctlTCSETS, unsafe.Pointer(&mod)); err != nil {
		return err
	}
	r.active = true
	return nil
}

// Restore puts the terminal back the way it was.
func (r *rawMode) Restore() {
	if !r.active {
		return
	}
	_ = ioctl(r.fd, ioctlTCSETS, unsafe.Pointer(&r.saved))
	r.active = false
}

// ioctlWinsize queries the window size for a file descriptor.
func ioctlWinsize(fd uintptr) (cols, rows int, err error) {
	var ws winsize
	if err := ioctl(int(fd), ioctlTIOCGWINSZ, unsafe.Pointer(&ws)); err != nil {
		return 80, 24, err
	}
	if ws.Cols == 0 {
		return 80, 24, fmt.Errorf("zero terminal width")
	}
	return int(ws.Cols), int(ws.Rows), nil
}

// terminalSize returns the current terminal columns and rows.
func terminalSize() (cols, rows int) {
	c, r, err := ioctlWinsize(0)
	if err != nil {
		return 80, 24
	}
	return c, r
}
