// Package terminal provides terminal manipulation functions
package terminal

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

// Termios holds terminal attributes (Linux struct termios layout)
type Termios struct {
	Iflag  uint32
	Oflag  uint32
	Cflag  uint32
	Lflag  uint32
	Line   uint8
	Cc     [32]uint8
	Ispeed uint32
	Ospeed uint32
}

// tcflag types
type tcflag uint32
type cc_t uint8
type speed_t uint32

// Save state for restoration
var savedTermios *Termios

// Ioctl constants
const (
	TCGETS = 0x5401
	TCSETS = 0x5402
)

// Terminal flags
const (
	ICANON tcflag = 0x00000002
	ECHO   tcflag = 0x00000008
	ISIG   tcflag = 0x00000001
	IXON   tcflag = 0x00000400
	ICRNL  tcflag = 0x00000100
	INLCR  tcflag = 0x00000040
	CS8    tcflag = 0x00000030
)

// VMIN and VTIME indices
const (
	VMIN  = 6
	VTIME = 5
)

// SetRawMode puts the terminal into raw mode
func SetRawMode() error {
	var termios Termios
	
	// Get current terminal settings
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(os.Stdin.Fd()),
		uintptr(TCGETS),
		uintptr(unsafe.Pointer(&termios)),
	)
	if errno != 0 {
		return fmt.Errorf("failed to get terminal settings: %v", errno)
	}
	
	// Save current settings
	savedTermios = &Termios{
		Iflag:  termios.Iflag,
		Oflag:  termios.Oflag,
		Cflag:  termios.Cflag,
		Lflag:  termios.Lflag,
		Line:   termios.Line,
		Ispeed: termios.Ispeed,
		Ospeed: termios.Ospeed,
	}
	copy(savedTermios.Cc[:], termios.Cc[:])
	
	// Set raw mode flags
	termios.Lflag &^= uint32(ICANON | ECHO | ISIG)
	termios.Iflag &^= uint32(IXON | ICRNL | INLCR)
	termios.Cflag |= uint32(CS8)
	termios.Cc[VMIN] = 1
	termios.Cc[VTIME] = 0
	
	// Set new terminal settings
	_, _, errno = syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(os.Stdin.Fd()),
		uintptr(TCSETS),
		uintptr(unsafe.Pointer(&termios)),
	)
	if errno != 0 {
		return fmt.Errorf("failed to set terminal settings: %v", errno)
	}
	
	return nil
}

// RestoreMode restores the terminal to its original settings
func RestoreMode() error {
	if savedTermios == nil {
		return nil
	}
	
	termios := Termios{
		Iflag:  savedTermios.Iflag,
		Oflag:  savedTermios.Oflag,
		Cflag:  savedTermios.Cflag,
		Lflag:  savedTermios.Lflag,
		Line:   savedTermios.Line,
		Ispeed: savedTermios.Ispeed,
		Ospeed: savedTermios.Ospeed,
	}
	copy(termios.Cc[:], savedTermios.Cc[:])
	
	_, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		uintptr(os.Stdin.Fd()),
		uintptr(TCSETS),
		uintptr(unsafe.Pointer(&termios)),
	)
	if errno != 0 {
		return fmt.Errorf("failed to restore terminal settings: %v", errno)
	}
	
	savedTermios = nil
	return nil
}

// IsRawModeRestored returns true if the terminal mode has been restored
func IsRawModeRestored() bool {
	return savedTermios == nil
}
