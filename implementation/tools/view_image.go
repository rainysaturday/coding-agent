// Package tools implements the tool execution system for the coding agent.
// This file contains the view_image tool implementation.
package tools

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"
)

// executeViewImage reads a local image file and returns it as a base64-encoded data URI
// so that the agent can send it to a vision-capable model for analysis.
func (te *ToolExecutor) executeViewImage(params map[string]interface{}) *ToolResult {
	path, ok := params["path"].(string)
	if !ok || path == "" {
		return &ToolResult{
			Success: false,
			Error:   "missing required parameter: path",
		}
	}

	// Extract optional custom prompt for vision analysis
	var customPrompt string
	if p, hasPrompt := params["prompt"]; hasPrompt {
		if str, ok := p.(string); ok {
			customPrompt = str
		}
	}

	// Read the file
	data, err := os.ReadFile(path)
	if err != nil {
		return &ToolResult{
			Success: false,
			Error:   formatFileError(err, path),
		}
	}

	// Validate file size (max 20MB to prevent memory issues)
	const maxImageSize = 20 * 1024 * 1024 // 20 MB
	if len(data) > maxImageSize {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("image file too large: %d bytes (max %d bytes)", len(data), maxImageSize),
		}
	}

	// Determine MIME type from file extension and content
	mimeType := detectImageMIMEType(path, data)
	if mimeType == "" {
		return &ToolResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported or unrecognized image format: %s", path),
		}
	}

	// Encode as base64 data URI
	encoded := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

	return &ToolResult{
		Success: true,
		Output:  fmt.Sprintf("Image loaded: %s (%s, %d bytes)", filepath.Base(path), mimeType, len(data)),
		Path:    path,
		Extra: map[string]interface{}{
			"data_uri":  dataURI,
			"mime_type": mimeType,
			"size":      len(data),
			"prompt":    customPrompt,
		},
	}
}

// detectImageMIMEType determines the MIME type of an image file.
// It first checks the file extension, then falls back to HTTP content type detection.
func detectImageMIMEType(path string, data []byte) string {
	// Allowed image MIME types
	allowedMIMETypes := map[string]bool{
		"image/png":  true,
		"image/jpeg": true,
		"image/gif":  true,
		"image/webp": true,
	}

	// First, try to detect from file extension
	ext := strings.ToLower(filepath.Ext(path))
	extMIME := map[string]string{
		".png":  "image/png",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".gif":  "image/gif",
		".webp": "image/webp",
	}[ext]

	if extMIME != "" && allowedMIMETypes[extMIME] {
		return extMIME
	}

	// Fall back to mime type detection (uses magic bytes)
	mimeType := mime.TypeByExtension(ext)
	if mimeType != "" && allowedMIMETypes[mimeType] {
		return mimeType
	}

	// Also check for specific magic bytes as a final fallback
	if len(data) >= 3 {
		// PNG: 89 50 4E 47
		if data[0] == 0x89 && data[1] == 0x50 && data[2] == 0x4E {
			return "image/png"
		}
		// JPEG: FF D8 FF
		if data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF {
			return "image/jpeg"
		}
		// GIF: 47 49 46 38
		if len(data) >= 4 && data[0] == 'G' && data[1] == 'I' && data[2] == 'F' && data[3] == '8' {
			return "image/gif"
		}
	}

	return ""
}

// ViewImageExtra contains extra data returned by view_image tool.
type ViewImageExtra struct {
	DataURI  string `json:"data_uri"`
	MIMEType string `json:"mime_type"`
	Size     int    `json:"size"`
	Prompt   string `json:"prompt,omitempty"`
}

// GetViewImageExtra extracts the view_image extra data from a ToolResult.
func GetViewImageExtra(result *ToolResult) *ViewImageExtra {
	if result == nil || result.Extra == nil {
		return nil
	}

	extra, ok := result.Extra["view_image_extra"]
	if !ok {
		// Try direct access to the fields
		dataURI, _ := result.Extra["data_uri"].(string)
		mimeType, _ := result.Extra["mime_type"].(string)
		var size int
		switch v := result.Extra["size"].(type) {
		case int:
			size = v
		case float64:
			size = int(v)
		}
		var prompt string
		if p, ok := result.Extra["prompt"].(string); ok {
			prompt = p
		}
		if dataURI != "" {
			return &ViewImageExtra{
				DataURI:  dataURI,
				MIMEType: mimeType,
				Size:     size,
				Prompt:   prompt,
			}
		}
		return nil
	}

	viewExtra, ok := extra.(*ViewImageExtra)
	if !ok {
		return nil
	}
	return viewExtra
}
