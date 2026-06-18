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

// executeListFiles lists files and directories, formatted like ls.
// Supports context cancellation and various flags similar to ls.
func (te *ToolExecutor) executeListFiles(ctx context.Context, params map[string]interface{}) *ToolResult {
	path := "."
	if p, ok := params["path"].(string); ok && p != "" {
		path = p
	}

	// Parse flags
	flags := map[string]bool{
		"l": false,
		"a": false,
		"h": false,
		"t": false,
		"S": false,
		"r": false,
		"R": false,
	}

	if flagsParam, ok := params["flags"]; ok {
		switch v := flagsParam.(type) {
		case []interface{}:
			for _, f := range v {
				if flagStr, ok := f.(string); ok {
					if len(flagStr) == 1 {
						flags[flagStr] = true
					}
				}
			}
		case []string:
			for _, flagStr := range v {
				if len(flagStr) == 1 {
					flags[flagStr] = true
				}
			}
		}
	}

	// Check if path is a file or directory
	info, err := os.Stat(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	// If it's a file, return information about that single file
	if !info.IsDir() {
		if flags["l"] {
			line := formatFileLong(info, flags)
			return &ToolResult{
				Success: true,
				Output:  line,
				Extra: map[string]interface{}{
					"entriesListed": 1,
					"path":          path,
				},
			}
		}
		return &ToolResult{
			Success: true,
			Output:  info.Name(),
			Extra: map[string]interface{}{
				"entriesListed": 1,
				"path":          path,
			},
		}
	}

	var entries []os.DirEntry
	var output string

	// Handle recursive listing
	if flags["R"] {
		// Use filepath.Walk to get entries with relative paths
		var resultEntries []walkEntry
		walkErr := filepath.Walk(path, func(filePath string, fileInfo os.FileInfo, walkErr error) error {
			if walkErr != nil {
				return nil
			}
			// Skip .git directories
			if strings.Contains(filePath, "/.git/") || strings.HasSuffix(filePath, "/.git") {
				if fileInfo.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			// Skip hidden files/dirs unless "a" flag is set
			if !flags["a"] {
				baseName := fileInfo.Name()
				if strings.HasPrefix(baseName, ".") {
					if fileInfo.IsDir() {
						return filepath.SkipDir
					}
					return nil
				}
			}
			relPath, _ := filepath.Rel(path, filePath)
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
			return &ToolResult{
				Success: false,
				Error:   formatFileError(walkErr, path),
			}
		}

		// Sort entries
		sort.Slice(resultEntries, func(i, j int) bool {
			// Directories first
			iIsDir := resultEntries[i].isDir
			jIsDir := resultEntries[j].isDir
			if iIsDir != jIsDir {
				return iIsDir
			}
			// Then by sort criteria
			switch {
			case flags["t"]:
				if flags["r"] {
					return resultEntries[i].modTime.Before(resultEntries[j].modTime)
				}
				return resultEntries[i].modTime.After(resultEntries[j].modTime)
			case flags["S"]:
				if flags["r"] {
					return resultEntries[i].fileSize < resultEntries[j].fileSize
				}
				return resultEntries[i].fileSize > resultEntries[j].fileSize
			default:
				if flags["r"] {
					return resultEntries[i].path > resultEntries[j].path
				}
				return resultEntries[i].path < resultEntries[j].path
			}
		})

		if flags["l"] {
			output = formatRecursiveLongList(resultEntries, path, flags)
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
			Extra: map[string]interface{}{
				"entriesListed": len(resultEntries),
				"path":          path,
			},
		}
	}

	// Read directory (non-recursive)
	entries, err = os.ReadDir(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	// Filter entries
	var filtered []os.DirEntry
	for _, entry := range entries {
		name := entry.Name()
		if !flags["a"] && strings.HasPrefix(name, ".") {
			continue
		}
		filtered = append(filtered, entry)
	}

	// Sort entries
	sort.Slice(filtered, func(i, j int) bool {
		// Directories first
		iIsDir := filtered[i].IsDir()
		jIsDir := filtered[j].IsDir()
		if iIsDir != jIsDir {
			return iIsDir
		}

		// Then by the specified sort criteria
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

	// Format output
	if flags["l"] {
		output = formatLongList(filtered, flags)
	} else {
		output = formatSimpleList(filtered)
	}

	return &ToolResult{
		Success: true,
		Output:  output,
		Extra: map[string]interface{}{
			"entriesListed": len(filtered),
			"path":          path,
		},
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
