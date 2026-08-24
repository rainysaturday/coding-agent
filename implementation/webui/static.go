package webui

import "embed"

//go:embed static/*
var staticFS embed.FS

// staticAsset returns the named embedded asset or an empty string if absent.
func staticAsset(name string) ([]byte, bool) {
	data, err := staticFS.ReadFile("static/" + name)
	if err != nil {
		return nil, false
	}
	return data, true
}
