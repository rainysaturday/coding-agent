//go:build !linux

package tui

import (
	"errors"
	"fmt"
	"os"
)

// rawMode is a no-op fallback on non-Linux platforms; the TUI then runs in
// cooked line mode without arrow-key editing.
type rawMode struct {
	fd     int
	active bool
}

func (r *rawMode) Enter() error { return errors.New("raw mode is only supported on Linux") }
func (r *rawMode) Restore()     {}

func ioctlWinsize(fd uintptr) (cols, rows int, err error) {
	if w, ok := os.LookupEnv("COLUMNS"); ok {
		var c int
		if _, err := fmt.Sscanf(w, "%d", &c); err == nil && c > 0 {
			return c, 24, nil
		}
	}
	return 80, 24, errors.New("window size unavailable")
}

func terminalSize() (cols, rows int) {
	c, r, _ := ioctlWinsize(0)
	return c, r
}
