package main

import "testing"

func TestNormalizeThemeChoiceDefaultsToProfessionalDark(t *testing.T) {
	for _, input := range []string{"", "unknown"} {
		if got := normalizeThemeChoice(input); got != themeChoiceProfessionalDark {
			t.Fatalf("normalizeThemeChoice(%q) = %q, want %q", input, got, themeChoiceProfessionalDark)
		}
	}
}

func TestThemeChoiceLabels(t *testing.T) {
	tests := map[string]string{
		themeChoiceProfessionalDark: "Professional Dark",
		themeChoiceSystem:           "System",
		themeChoiceLight:            "Light",
		themeChoiceFyneDark:         "Fyne Dark",
		"dark":                      "Fyne Dark",
	}
	for input, want := range tests {
		if got := themeLabel(input); got != want {
			t.Fatalf("themeLabel(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestThemeChoiceFromLabel(t *testing.T) {
	if got := themeChoiceFromLabel("Fyne Dark"); got != themeChoiceFyneDark {
		t.Fatalf("themeChoiceFromLabel(Fyne Dark) = %q", got)
	}
	if got := themeChoiceFromLabel("Professional Dark"); got != themeChoiceProfessionalDark {
		t.Fatalf("themeChoiceFromLabel(Professional Dark) = %q", got)
	}
}
