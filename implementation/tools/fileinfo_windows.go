//go:build windows
// +build windows

package tools

// getFileInfoDetails returns placeholder values for Windows (ownership info not directly available)
func getFileInfoDetails(sys interface{}) (linkCount, owner, group string) {
	return "1", "?", "?"
}
