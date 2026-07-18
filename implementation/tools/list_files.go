// Package tools implements the tool execution system for the coding agent.
// This file contains the list_files tool implementation.
package tools

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// walkEntry holds a directory entry with its metadata for recursive listing.
type walkEntry struct {
	path     string
	isDir    bool
	info     os.FileInfo
	modTime  time.Time
	fileSize int64
}

// listFilesParams holds the parsed parameters for a list_files operation.
type listFilesParams struct {
	path  string
	flags map[string]bool
}

// parseListFilesParams extracts and validates list_files parameters from the tool params map.
func parseListFilesParams(params map[string]interface{}) *listFilesParams {
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	flags := parseFlagsParamToMap(params)
	for _, k := range []string{"l", "a", "h", "t", "S", "r", "R"} {
		if _, ok := flags[k]; !ok {
			flags[k] = false
		}
	}

	return &listFilesParams{path: path, flags: flags}
}

// sortDirEntries sorts directory entries by sort criteria.
func sortDirEntries(filtered []os.DirEntry, flags map[string]bool) {
	sort.Slice(filtered, func(i, j int) bool {
		iIsDir := filtered[i].IsDir()
		jIsDir := filtered[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}
		switch {
		case flags["t"]:
			iInfo, _ := filtered[i].Info()
			jInfo, _ := filtered[j].Info()
			if flags["r"] {
				return iInfo.ModTime().Before(jInfo.ModTime())
			}
			return iInfo.ModTime().After(jInfo.ModTime())
		case flags["S"]:
			iInfo, _ := filtered[i].Info()
			jInfo, _ := filtered[j].Info()
			if flags["r"] {
				return iInfo.Size() < jInfo.Size()
			}
			return iInfo.Size() > jInfo.Size()
		default:
			if flags["r"] {
				return filtered[i].Name() > filtered[j].Name()
			}
			return filtered[i].Name() < filtered[j].Name()
		}
	})
}

// sortWalkEntries sorts walk entries by sort criteria.
func sortWalkEntries(entries []walkEntry, flags map[string]bool) {
	sort.Slice(entries, func(i, j int) bool {
		iIsDir := entries[i].isDir
		jIsDir := entries[j].isDir
		if iIsDir != jIsDir {
			return iIsDir
		}
		switch {
		case flags["t"]:
			if flags["r"] {
				return entries[i].modTime.Before(entries[j].modTime)
			}
			return entries[i].modTime.After(entries[j].modTime)
		case flags["S"]:
			if flags["r"] {
				return entries[i].fileSize < entries[j].fileSize
			}
			return entries[i].fileSize > entries[j].fileSize
		default:
			if flags["r"] {
				return entries[i].path > entries[j].path
			}
			return entries[i].path < entries[j].path
		}
	})
}

// filterHiddenEntries filters out hidden files/directories unless "a" flag is set.
func filterHiddenEntries(entries []os.DirEntry, showHidden bool) []os.DirEntry {
	if showHidden {
		return entries
	}
	var filtered []os.DirEntry
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), ".") {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

// executeListFiles lists files and directories, formatted like ls.
// Supports context cancellation and various flags similar to ls.
func (te *ToolExecutor) executeListFiles(ctx context.Context, params map[string]interface{}) *ToolResult {
	lp := parseListFilesParams(params)

	info, err := os.Stat(lp.path)
	if err != nil {
		return &ToolResult{Success: false, Error: formatFileError(err, lp.path)}
	}

	// Handle single file
	if !info.IsDir() {
		output := info.Name()
		if lp.flags["l"] {
			output = formatFileLong(info, lp.flags)
		}
		return &ToolResult{
			Success: true,
			Output:  output,
			Extra:   map[string]interface{}{"entriesListed": 1, "path": lp.path},
		}
	}

	// Handle recursive listing
	if lp.flags["R"] {
		return te.listFilesRecursive(lp)
	}

	// Handle non-recursive listing
	return te.listFilesNonRecursive(lp)
}

// listFilesRecursive handles recursive directory listing.
func (te *ToolExecutor) listFilesRecursive(lp *listFilesParams) *ToolResult {
	var resultEntries []walkEntry
	walkErr := filepath.Walk(lp.path, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if strings.Contains(filePath, "/.git/") || strings.HasSuffix(filePath, "/.git") {
			if fileInfo.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !lp.flags["a"] {
			if strings.HasPrefix(fileInfo.Name(), ".") {
				if fileInfo.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
		}
		relPath, _ := filepath.Rel(lp.path, filePath)
		resultEntries = append(resultEntries, walkEntry{
			path:     relPath,
			isDir:    fileInfo.IsDir(),
			info:     fileInfo,
			modTime:  fileInfo.ModTime(),
			fileSize: fileInfo.Size(),
		})
		return nil
	})
	if walkErr != nil {
		return &ToolResult{Success: false, Error: formatFileError(walkErr, lp.path)}
	}

	sortWalkEntries(resultEntries, lp.flags)

	var output string
	if lp.flags["l"] {
		output = formatRecursiveLongList(resultEntries, lp.path, lp.flags)
	} else {
		var names []string
		for _, e := range resultEntries {
			name := e.path
			if e.isDir {
				name += "/"
			}
			names = append(names, name)
		}
		output = strings.Join(names, "\n")
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra:   map[string]interface{}{"entriesListed": len(resultEntries), "path": lp.path},
	}
}

// listFilesNonRecursive handles non-recursive directory listing.
func (te *ToolExecutor) listFilesNonRecursive(lp *listFilesParams) *ToolResult {
	entries, err := os.ReadDir(lp.path)
	if err != nil {
		return &ToolResult{Success: false, Error: formatFileError(err, lp.path)}
	}

	filtered := filterHiddenEntries(entries, lp.flags["a"])
	sortDirEntries(filtered, lp.flags)

	var output string
	if lp.flags["l"] {
		output = formatLongList(filtered, lp.flags)
	} else {
		output = formatSimpleList(filtered)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra:   map[string]interface{}{"entriesListed": len(filtered), "path": lp.path},
	}
}

// formatSimpleList returns a simple one-per-line listing (like `ls`).
func formatSimpleList(entries []os.DirEntry) string {
	var lines []string
	for _, entry := range entries {
		if entry.IsDir() {
			lines = append(lines, entry.Name()+"/")
		} else {
			lines = append(lines, entry.Name())
		}
	}
	return strings.Join(lines, "\n")
}

// formatLongList returns a long-format listing (like `ls -l`).
func formatLongList(entries []os.DirEntry, flags map[string]bool) string {
	var lines []string
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}
		lines = append(lines, formatFileLong(info, flags))
	}
	return strings.Join(lines, "\n")
}

// formatFileLong returns a long-format line for a single file info.
func formatFileLong(info os.FileInfo, flags map[string]bool) string {
	// Permissions
	permStr := formatPermissions(info)

	// Size
	size := info.Size()
	var sizeStr string
	if flags["h"] {
		sizeStr = humanReadableSize(size)
	} else {
		sizeStr = fmt.Sprintf("%d", size)
	}

	// Modification time (format: "Jan 02 15:04" or "Jan 02 2006" if old)
	modTime := info.ModTime()
	now := time.Now()
	age := now.Sub(modTime)
	var timeStr string
	if age > 365*24*time.Hour {
		timeStr = modTime.Format("Jan 02  2006")
	} else {
		timeStr = modTime.Format("Jan 02 15:04")
	}

	// Name (with / suffix for directories)
	name := info.Name()
	if info.IsDir() {
		name += "/"
	}

	// Get ownership info via platform-specific function
	linkCount, owner, group := getFileInfoDetails(info.Sys())

	// Format: permissions links owner group size timestamp name
	return fmt.Sprintf("%s  %s  %s  %s  %s  %s  %s", permStr, linkCount, owner, group, sizeStr, timeStr, name)
}

// formatRecursiveLongList formats a list of walkEntries for recursive long-format output.
func formatRecursiveLongList(entries []walkEntry, basePath string, flags map[string]bool) string {
	var lines []string
	for _, e := range entries {
		// Permissions
		permStr := formatPermissionsRecursive(e)

		// Size
		var sizeStr string
		if flags["h"] {
			sizeStr = humanReadableSize(e.fileSize)
		} else {
			sizeStr = fmt.Sprintf("%d", e.fileSize)
		}

		// Modification time
		now := time.Now()
		age := now.Sub(e.modTime)
		var timeStr string
		if age > 365*24*time.Hour {
			timeStr = e.modTime.Format("Jan 02  2006")
		} else {
			timeStr = e.modTime.Format("Jan 02 15:04")
		}

		// Name (with / suffix for directories)
		name := e.path
		if e.isDir {
			name += "/"
		}

		lines = append(lines, fmt.Sprintf("%s  1  ?  ?  %s  %s  %s", permStr, sizeStr, timeStr, name))
	}
	return strings.Join(lines, "\n")
}

// formatPermissionsRecursive returns a Unix-style permission string for recursive listing.
func formatPermissionsRecursive(e walkEntry) string {
	mode := e.info.Mode()

	var fileType byte
	switch {
	case mode.IsDir():
		fileType = 'd'
	case mode&os.ModeSymlink != 0:
		fileType = 'l'
	case mode.IsRegular():
		fileType = '-'
	default:
		fileType = '-'
	}

	perm := mode.Perm()
	var permStr bytes.Buffer
	permStr.WriteByte(fileType)

	for _, bit := range []struct {
		set   string
		clear string
		mode  os.FileMode
	}{
		{"r", "-", 0400},
		{"w", "-", 0200},
		{"x", "-", 0100},
		{"r", "-", 0040},
		{"w", "-", 0020},
		{"x", "-", 0010},
		{"r", "-", 0004},
		{"w", "-", 0002},
		{"x", "-", 0001},
	} {
		if perm&bit.mode != 0 {
			permStr.WriteString(bit.set)
		} else {
			permStr.WriteString(bit.clear)
		}
	}

	return permStr.String()
}

// formatPermissions returns a Unix-style permission string.
func formatPermissions(info os.FileInfo) string {
	mode := info.Mode()

	// File type
	var fileType byte
	switch {
	case mode.IsDir():
		fileType = 'd'
	case mode&os.ModeSymlink != 0:
		fileType = 'l'
	case mode.IsRegular():
		fileType = '-'
	default:
		fileType = '-'
	}

	result := string(fileType)
	// Owner permissions
	result += formatTriple(uint8((mode >> 6) & 07))
	// Group permissions
	result += formatTriple(uint8((mode >> 3) & 07))
	// Other permissions
	result += formatTriple(uint8(mode & 07))
	return result
}

// formatTriple formats three permission bits as rwx.
func formatTriple(perm uint8) string {
	var s string
	if perm&4 != 0 {
		s += "r"
	} else {
		s += "-"
	}
	if perm&2 != 0 {
		s += "w"
	} else {
		s += "-"
	}
	if perm&1 != 0 {
		s += "x"
	} else {
		s += "-"
	}
	return s
}

// humanReadableSize converts a byte count to human-readable format.
func humanReadableSize(size int64) string {
	const (
		KB = 1 << 10
		MB = 1 << 20
		GB = 1 << 30
	)

	switch {
	case size >= GB:
		return fmt.Sprintf("%.1fG", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.1fM", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.1fK", float64(size)/KB)
	default:
		return fmt.Sprintf("%d", size)
	}
}
