package colors

import (
	"strings"
	"testing"
)

// ===== SetTheme / ApplyTheme =====

func TestSetTheme_ValidThemes(t *testing.T) {
	themes := ListThemes()
	if len(themes) != 5 {
		t.Errorf("Expected 5 built-in themes, got %d: %v", len(themes), themes)
	}

	for _, name := range themes {
		t.Run(name, func(t *testing.T) {
			ResetTheme() // reset
			if err := SetTheme(name); err != nil {
				t.Errorf("SetTheme(%q) returned error: %v", name, err)
			}
			theme := GetCurrentTheme()
			if theme == nil {
				t.Fatalf("GetCurrentTheme() returned nil after SetTheme(%q)", name)
			}
			if theme.Name != name {
				t.Errorf("Expected theme name %q, got %q", name, theme.Name)
			}
		})
	}
}

func TestSetTheme_InvalidTheme(t *testing.T) {
	ResetTheme()
	err := SetTheme("nonexistent")
	if err == nil {
		t.Error("Expected error for invalid theme, got nil")
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("Expected error to mention theme name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "valid themes are") {
		t.Errorf("Expected error to list valid themes, got: %v", err)
	}
}

func TestSetTheme_EmptyString(t *testing.T) {
	ResetTheme()
	err := SetTheme("")
	if err == nil {
		t.Error("Expected error for empty theme name, got nil")
	}
}

func TestApplyTheme(t *testing.T) {
	ResetTheme()
	err := ApplyTheme("light")
	if err != nil {
		t.Errorf("ApplyTheme('light') returned error: %v", err)
	}
	theme := GetCurrentTheme()
	if theme == nil || theme.Name != "light" {
		t.Errorf("Expected light theme after ApplyTheme, got: %v", theme)
	}
}

func TestApplyTheme_Invalid(t *testing.T) {
	ResetTheme()
	err := ApplyTheme("bogus")
	if err == nil {
		t.Error("Expected error from ApplyTheme with invalid name")
	}
}

// ===== GetColor =====

func TestGetColor_NoThemeSet(t *testing.T) {
	ResetTheme()
	// With no theme set, GetColor should return built-in defaults
	got := GetColor("reset")
	if got != ColorReset {
		t.Errorf("GetColor('reset') with no theme = %q, want %q", got, ColorReset)
	}
	got = GetColor("dim")
	if got != ColorDim {
		t.Errorf("GetColor('dim') with no theme = %q, want %q", got, ColorDim)
	}
	got = GetColor("red")
	if got != ColorRed {
		t.Errorf("GetColor('red') with no theme = %q, want %q", got, ColorRed)
	}
}

func TestGetColor_ValidSlots(t *testing.T) {
	slots := []string{"reset", "dim", "red", "green", "yellow", "blue", "magenta", "cyan"}
	for _, themeName := range ListThemes() {
		t.Run(themeName, func(t *testing.T) {
			ResetTheme()
			if err := SetTheme(themeName); err != nil {
				t.Fatalf("SetTheme(%q) failed: %v", themeName, err)
			}
			for _, slot := range slots {
				got := GetColor(slot)
				if got == "" {
					t.Errorf("GetColor(%q) returned empty string for theme %q", slot, themeName)
				}
				if got == ColorReset && slot != "reset" {
					// Only "reset" slot should return ColorReset when no theme
					// With a theme set, it should return the theme's reset
					theme := BuiltInThemes[themeName]
					if slot == "reset" && got != theme.Reset {
						t.Errorf("GetColor('reset') = %q, want theme reset %q", got, theme.Reset)
					}
				}
			}
		})
	}
}

func TestGetColor_UnknownSlot(t *testing.T) {
	ResetTheme()
	if err := SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	// Unknown slot should return ColorReset (fallback)
	got := GetColor("unknown_slot")
	if got != ColorReset {
		t.Errorf("GetColor('unknown_slot') = %q, want %q (ColorReset)", got, ColorReset)
	}
}

func TestGetColor_AllSlotsNonEmpty(t *testing.T) {
	slots := []string{"reset", "dim", "red", "green", "yellow", "blue", "magenta", "cyan"}
	for _, themeName := range ListThemes() {
		t.Run(themeName, func(t *testing.T) {
			ResetTheme()
			if err := SetTheme(themeName); err != nil {
				t.Fatalf("SetTheme(%q) failed: %v", themeName, err)
			}
			for _, slot := range slots {
				got := GetColor(slot)
				if got == "" {
					t.Errorf("Theme %q: GetColor(%q) is empty", themeName, slot)
				}
			}
		})
	}
}

func TestGetColor_ThemesProduceDifferentColors(t *testing.T) {
	// Verify that different themes produce different color codes for at least one slot
	slots := []string{"red", "green", "yellow", "blue", "magenta", "cyan"}
	themeNames := ListThemes()
	if len(themeNames) < 2 {
		t.Fatal("Need at least 2 themes to compare")
	}

	ResetTheme()
	if err := SetTheme(themeNames[0]); err != nil {
		t.Fatal(err)
	}
	first := GetColor("red")

	ResetTheme()
	if err := SetTheme(themeNames[1]); err != nil {
		t.Fatal(err)
	}
	second := GetColor("red")

	// The two themes should produce at least some different color codes
	foundDifference := false
	for _, slot := range slots {
		ResetTheme()
		if err := SetTheme(themeNames[0]); err != nil {
			t.Fatal(err)
		}
		a := GetColor(slot)

		ResetTheme()
		if err := SetTheme(themeNames[1]); err != nil {
			t.Fatal(err)
		}
		b := GetColor(slot)

		if a != b {
			foundDifference = true
			break
		}
	}
	if !foundDifference {
		t.Errorf("Expected at least two themes to differ in some color slot, but %q and %q produced identical codes", themeNames[0], themeNames[1])
	}
	_ = first
	_ = second
}

// ===== ListThemes =====

func TestListThemes(t *testing.T) {
	themes := ListThemes()
	expected := []string{"dark", "light", "solarized", "gruvbox", "darkula"}

	if len(themes) != len(expected) {
		t.Fatalf("Expected %d themes, got %d: %v", len(expected), len(themes), themes)
	}

	themeSet := make(map[string]bool)
	for _, t := range themes {
		themeSet[t] = true
	}
	for _, e := range expected {
		if !themeSet[e] {
			t.Errorf("Expected theme %q in list, got: %v", e, themes)
		}
	}
}

func TestListThemes_ReturnsCopy(t *testing.T) {
	// Verify ListThemes returns a new slice each time (caller can modify safely)
	a := ListThemes()
	b := ListThemes()
	if len(a) > 0 {
		a[0] = "modified"
	}
	if b[0] == "modified" {
		t.Error("ListThemes should return independent slices")
	}
}

// ===== GetCurrentTheme =====

func TestGetCurrentTheme_ReturnsCopy(t *testing.T) {
	ResetTheme()
	if err := SetTheme("dark"); err != nil {
		t.Fatal(err)
	}
	theme1 := GetCurrentTheme()
	if theme1 == nil {
		t.Fatal("GetCurrentTheme() returned nil")
	}
	theme1.Name = "tampered"
	theme2 := GetCurrentTheme()
	if theme2.Name == "tampered" {
		t.Error("GetCurrentTheme() should return a copy, not the internal pointer")
	}
}

func TestGetCurrentTheme_NilWhenNotSet(t *testing.T) {
	ResetTheme()
	theme := GetCurrentTheme()
	if theme != nil {
		t.Errorf("Expected nil when no theme set, got: %v", theme)
	}
}

// ===== Built-in theme integrity =====

func TestBuiltInThemes_AllSlotsDefined(t *testing.T) {
	requiredSlots := []string{"Name", "Reset", "Dim", "Red", "Green", "Yellow", "Blue", "Magenta", "Cyan"}
	for _, themeName := range ListThemes() {
		t.Run(themeName, func(t *testing.T) {
			theme := BuiltInThemes[themeName]
			for _, slot := range requiredSlots {
				v := fieldValue(theme, slot)
				if v == "" {
					t.Errorf("Theme %q: field %q is empty", themeName, slot)
				}
			}
		})
	}
}

func TestBuiltInThemes_NoDuplicateNames(t *testing.T) {
	seen := make(map[string]bool)
	for name := range BuiltInThemes {
		if seen[name] {
			t.Errorf("Duplicate theme name: %q", name)
		}
		seen[name] = true
	}
}

// fieldValue is a test helper to access struct fields by name.
func fieldValue(theme Theme, field string) string {
	switch field {
	case "Name":
		return theme.Name
	case "Reset":
		return theme.Reset
	case "Dim":
		return theme.Dim
	case "Red":
		return theme.Red
	case "Green":
		return theme.Green
	case "Yellow":
		return theme.Yellow
	case "Blue":
		return theme.Blue
	case "Magenta":
		return theme.Magenta
	case "Cyan":
		return theme.Cyan
	}
	return ""
}
