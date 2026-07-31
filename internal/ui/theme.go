package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/Serge-Nook/kuznica/internal/config"
)

// forcedVariant wraps the default theme and pins the light or dark variant.
type forcedVariant struct {
	fyne.Theme
	variant fyne.ThemeVariant
}

func (f forcedVariant) Color(name fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
	return f.Theme.Color(name, f.variant)
}

// applyTheme switches the application theme according to the settings.
func applyTheme(app fyne.App, preference config.Theme) {
	switch preference {
	case config.ThemeLight:
		app.Settings().SetTheme(forcedVariant{Theme: theme.DefaultTheme(), variant: theme.VariantLight})
	case config.ThemeDark:
		app.Settings().SetTheme(forcedVariant{Theme: theme.DefaultTheme(), variant: theme.VariantDark})
	default:
		app.Settings().SetTheme(theme.DefaultTheme())
	}
}
