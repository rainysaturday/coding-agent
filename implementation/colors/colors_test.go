package colors

import (
	"strings"
	"testing"
)

func TestBuiltInThemesComplete(t *testing.T) {
	want := []string{"dark", "light", "solarized", "gruvbox", "darkula"}
	for _, name := range want {
		if _, ok := BuiltInThemes[name]; !ok {
			t.Fatalf("theme %q missing", name)
		}
	}
	if len(BuiltInThemes) != len(want) {
		t.Fatalf("expected %d themes, got %d", len(want), len(BuiltInThemes))
	}
}

func TestThemeSlotsNonEmptyAndDistinct(t *testing.T) {
	for name, th := range BuiltInThemes {
		seen := map[string]string{}
		for _, slot := range th.Slots() {
			v := th.Get(slot)
			if v == "" {
				t.Errorf("theme %s slot %s is empty", name, slot)
			}
			if slot == "reset" {
				continue
			}
			if prev, ok := seen[v]; ok && prev != slot {
				t.Errorf("theme %s: slots %s and %s share the same color %q", name, prev, slot, v)
			}
			seen[v] = slot
		}
	}
}

func TestSetThemeValidInvalid(t *testing.T) {
	defer SetTheme(DefaultTheme)
	if err := SetTheme("gruvbox"); err != nil {
		t.Fatalf("valid theme rejected: %v", err)
	}
	if GetCurrentTheme().Name != "gruvbox" {
		t.Fatalf("current theme not updated")
	}
	if err := SetTheme("not-a-theme"); err == nil {
		t.Fatal("invalid theme accepted")
	}
	if !strings.Contains(GetColor("nope"), "0m") {
		t.Fatal("unknown slot should return reset")
	}
}

func TestApplyThemeSetsColors(t *testing.T) {
	defer ApplyTheme(DefaultTheme)
	if err := ApplyTheme("light"); err != nil {
		t.Fatal(err)
	}
	if GetColor("red") != BuiltInThemes["light"].Red {
		t.Fatal("GetColor did not follow ApplyTheme")
	}
}

func TestListThemes(t *testing.T) {
	list := ListThemes()
	if len(list) != 5 {
		t.Fatalf("expected 5 themes, got %d", len(list))
	}
	for _, want := range []string{"dark", "light", "solarized", "gruvbox", "darkula"} {
		found := false
		for _, got := range list {
			if got == want {
				found = true
			}
		}
		if !found {
			t.Fatalf("ListThemes missing %s", want)
		}
	}
}

func TestValidTheme(t *testing.T) {
	if !ValidTheme("darkula") {
		t.Fatal("darkula should be valid")
	}
	if ValidTheme("blue") {
		t.Fatal("blue should be invalid")
	}
}

func TestPaint(t *testing.T) {
	out := Paint("red", "x")
	if !strings.Contains(out, "x") || !strings.HasSuffix(out, GetColor("reset")) {
		t.Fatalf("unexpected paint output %q", out)
	}
}

func TestCSSVariables(t *testing.T) {
	th := BuiltInThemes["dark"]
	css := th.CSSVariables()
	for _, slot := range []string{"--dim", "--red", "--green", "--yellow", "--blue", "--magenta", "--cyan"} {
		if !strings.Contains(css, slot) {
			t.Fatalf("CSS variables missing %s: %s", slot, css)
		}
	}
}
