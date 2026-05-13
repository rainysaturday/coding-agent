//go:build !windows
// +build !windows

package tools

import (
	"fmt"
	"syscall"
)

// getFileInfoDetails returns link count, owner, and group for Unix-like systems
func getFileInfoDetails(sys interface{}) (linkCount, owner, group string) {
	linkCount = "1"
	owner = "?"
	group = "?"
	if stat, ok := sys.(*syscall.Stat_t); ok {
		linkCount = fmt.Sprintf("%d", stat.Nlink)
		owner = fmt.Sprintf("%d", stat.Uid)
		group = fmt.Sprintf("%d", stat.Gid)
	}
	return
}
