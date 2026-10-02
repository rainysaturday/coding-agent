package tools

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// ImageViewer sends an image to a vision capable model and returns a
// description (implemented by the agent).
type ImageViewer interface {
	DescribeImage(ctx context.Context, dataURI, prompt string) (string, error)
}

// ViewImageTool reads a local image and asks a vision model about it.
type ViewImageTool struct {
	run    *RunContext
	viewer ImageViewer
}

// NewViewImageTool creates the view_image tool.
func NewViewImageTool(run *RunContext, viewer ImageViewer) *ViewImageTool {
	return &ViewImageTool{run: run, viewer: viewer}
}

func (t *ViewImageTool) Name() string { return "view_image" }

func (t *ViewImageTool) Description() string {
	return "View a local image file. Reads the image from disk and sends it to a vision-capable model for analysis. Returns a description of the image contents. Supported formats: PNG, JPEG, WEBP, GIF."
}

func (t *ViewImageTool) Schema() map[string]interface{} {
	return objectSchema(map[string]interface{}{
		"path":   prop("string", "Path to the image file to view"),
		"prompt": prop("string", "Optional custom prompt or question to guide the vision analysis. When provided, this prompt is sent to the vision model instead of the default description prompt."),
	}, []string{"path"})
}

func (t *ViewImageTool) ReadOnly() bool { return true }

var supportedImageTypes = map[string]string{
	"image/png":  "PNG",
	"image/jpeg": "JPEG",
	"image/webp": "WEBP",
	"image/gif":  "GIF",
}

// detectImageMime sniffs the image type from its magic bytes.
func detectImageMime(data []byte) string {
	switch {
	case len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n":
		return "image/png"
	case len(data) >= 3 && data[0] == 0xFF && data[1] == 0xD8 && data[2] == 0xFF:
		return "image/jpeg"
	case len(data) >= 12 && string(data[:4]) == "RIFF" && string(data[8:12]) == "WEBP":
		return "image/webp"
	case len(data) >= 6 && (string(data[:6]) == "GIF87a" || string(data[:6]) == "GIF89a"):
		return "image/gif"
	}
	return ""
}

// maxImageBytesDefault is the 20 MB limit from requirement 041.
const maxImageBytesDefault = 20 * 1024 * 1024

// defaultVisionPrompt is used when the model omits the prompt parameter.
const defaultVisionPrompt = "Describe this image in detail. Include any text visible in the image, objects, colors, layout, and any other relevant details."

func (t *ViewImageTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	path := getString(args, "path")
	if path == "" {
		return "", errors.New("missing required parameter: path")
	}
	if t.viewer == nil {
		return "", errors.New("image viewing is not available (no vision capable model configured)")
	}
	abs := t.run.resolvePath(path)
	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}
	if info.IsDir() {
		return "", fmt.Errorf("file not found: %s (is a directory)", path)
	}
	maxBytes := t.run.MaxImageBytes
	if maxBytes <= 0 {
		maxBytes = maxImageBytesDefault
	}
	if info.Size() > maxBytes {
		return "", fmt.Errorf("image file too large: %d bytes (max %d bytes)", info.Size(), maxBytes)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", fmt.Errorf("file not found: %s", path)
	}

	// MIME detection order (requirement 041): file extension first, then
	// HTTP content sniffing, then manual magic-byte checks.
	mime := mimeFromExtension(abs)
	if mime == "" {
		if sniffed := http.DetectContentType(data[:min(512, len(data))]); supportedImageTypes[sniffed] != "" {
			mime = sniffed
		}
	}
	if mime == "" {
		mime = detectImageMime(data)
	}
	if _, ok := supportedImageTypes[mime]; !ok {
		return "", fmt.Errorf("unsupported or unrecognized image format: %s", path)
	}

	dataURI := "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data)
	prompt := strings.TrimSpace(getString(args, "prompt"))
	if prompt == "" {
		prompt = defaultVisionPrompt
	}
	out, err := t.viewer.DescribeImage(ctx, dataURI, prompt)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(out) == "" {
		return "(the vision model returned no description)", nil
	}
	return out, nil
}

func mimeFromExtension(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".webp":
		return "image/webp"
	case ".gif":
		return "image/gif"
	}
	return ""
}
