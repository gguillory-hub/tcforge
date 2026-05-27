package main

const (
	themeChoiceProfessionalDark = "professional-dark"
	themeChoiceSystem           = "system"
	themeChoiceLight            = "light"
	themeChoiceFyneDark         = "fyne-dark"
)

func normalizeThemeChoice(choice string) string {
	switch choice {
	case themeChoiceSystem, themeChoiceLight, themeChoiceFyneDark, themeChoiceProfessionalDark:
		return choice
	case "dark":
		return themeChoiceFyneDark
	default:
		return themeChoiceProfessionalDark
	}
}

func themeLabel(choice string) string {
	switch normalizeThemeChoice(choice) {
	case themeChoiceSystem:
		return "System"
	case themeChoiceLight:
		return "Light"
	case themeChoiceFyneDark:
		return "Fyne Dark"
	default:
		return "Professional Dark"
	}
}

func themeChoiceFromLabel(label string) string {
	switch label {
	case "System":
		return themeChoiceSystem
	case "Light":
		return themeChoiceLight
	case "Fyne Dark":
		return themeChoiceFyneDark
	default:
		return themeChoiceProfessionalDark
	}
}
