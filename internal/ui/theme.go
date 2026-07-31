package ui

import (
	"image/color"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/Serge-Nook/kuznica/internal/config"
)

// compactSizes keep the interface usable on 1280x800 displays: the default
// Fyne metrics are roughly a third larger.
var compactSizes = map[fyne.ThemeSizeName]float32{
	theme.SizeNameText:               11,
	theme.SizeNameHeadingText:        15,
	theme.SizeNameSubHeadingText:     12,
	theme.SizeNameCaptionText:        9,
	theme.SizeNamePadding:            3,
	theme.SizeNameInnerPadding:       5,
	theme.SizeNameLineSpacing:        3,
	theme.SizeNameInlineIcon:         15,
	theme.SizeNameScrollBar:          10,
	theme.SizeNameScrollBarSmall:     4,
	theme.SizeNameSeparatorThickness: 1,
	theme.SizeNameInputBorder:        1,
}

// compactTheme shrinks the default theme metrics and optionally pins the
// light or dark colour variant.
type compactTheme struct {
	fyne.Theme
	variant *fyne.ThemeVariant
	scale   float32
}

func (c compactTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	if c.variant != nil {
		variant = *c.variant
	}
	return c.Theme.Color(name, variant)
}

func (c compactTheme) Size(name fyne.ThemeSizeName) float32 {
	size, ok := compactSizes[name]
	if !ok {
		size = c.Theme.Size(name)
	}
	scaled := size * c.scale
	if scaled < 1 && size >= 1 {
		return 1
	}
	return scaled
}

// applyTheme switches the application theme according to the settings.
func applyTheme(app fyne.App, cfg config.Config) {
	custom := compactTheme{Theme: theme.DefaultTheme(), scale: float32(cfg.NormalizedUIScale())}
	switch cfg.Theme {
	case config.ThemeLight:
		variant := theme.VariantLight
		custom.variant = &variant
	case config.ThemeDark:
		variant := theme.VariantDark
		custom.variant = &variant
	}
	app.Settings().SetTheme(custom)
}
