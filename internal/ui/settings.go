package ui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Serge-Nook/kuznica/internal/config"
	"github.com/Serge-Nook/kuznica/internal/i18n"
)

func (u *UI) showSettings() {
	themeOptions := []string{
		u.tr.T("settings.theme.sys"),
		u.tr.T("settings.theme.light"),
		u.tr.T("settings.theme.dark"),
	}
	themeByOption := map[string]config.Theme{
		themeOptions[0]: config.ThemeSystem,
		themeOptions[1]: config.ThemeLight,
		themeOptions[2]: config.ThemeDark,
	}
	themeSelect := widget.NewSelect(themeOptions, nil)
	switch u.cfg.Theme {
	case config.ThemeLight:
		themeSelect.SetSelected(themeOptions[1])
	case config.ThemeDark:
		themeSelect.SetSelected(themeOptions[2])
	default:
		themeSelect.SetSelected(themeOptions[0])
	}

	languageOptions := make([]string, 0, len(i18n.Languages))
	languageByOption := map[string]string{}
	for _, code := range i18n.Languages {
		name := i18n.LanguageNames[code]
		languageOptions = append(languageOptions, name)
		languageByOption[name] = code
	}
	languageSelect := widget.NewSelect(languageOptions, nil)
	languageSelect.SetSelected(i18n.LanguageNames[u.tr.Language()])

	makepkg := widget.NewCheck(u.tr.T("settings.makepkg"), nil)
	makepkg.SetChecked(u.cfg.UseMakepkg)
	cleanup := widget.NewCheck(u.tr.T("settings.cleanup"), nil)
	cleanup.SetChecked(u.cfg.RemoveTempFiles)
	autoDeps := widget.NewCheck(u.tr.T("settings.autodeps"), nil)
	autoDeps.SetChecked(u.cfg.AutoInstallDeps)
	createDesktop := widget.NewCheck(u.tr.T("settings.desktop"), nil)
	createDesktop.SetChecked(u.cfg.CreateDesktop)
	validateDesktop := widget.NewCheck(u.tr.T("settings.validate"), nil)
	validateDesktop.SetChecked(u.cfg.ValidateDesktop)
	detectIcons := widget.NewCheck(u.tr.T("settings.icons"), nil)
	detectIcons.SetChecked(u.cfg.AutoDetectIcons)
	showLog := widget.NewCheck(u.tr.T("settings.showlog"), nil)
	showLog.SetChecked(u.cfg.ShowLogAfterMake)

	outputDir := widget.NewEntry()
	outputDir.SetText(u.cfg.OutputDir)

	general := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem(u.tr.T("settings.theme"), themeSelect),
			widget.NewFormItem(u.tr.T("settings.language"), languageSelect),
			widget.NewFormItem(u.tr.T("settings.output"), outputDir),
		),
	)
	work := container.NewVBox(makepkg, cleanup, autoDeps, createDesktop, validateDesktop, detectIcons, showLog)

	tabs := container.NewAppTabs(
		container.NewTabItem(u.tr.T("settings.general"), general),
		container.NewTabItem(u.tr.T("settings.work"), work),
	)

	form := dialog.NewCustomConfirm(u.tr.T("settings.title"), u.tr.T("button.save"), u.tr.T("button.cancel"), tabs,
		func(save bool) {
			if !save {
				return
			}
			previousLanguage := u.cfg.Language
			u.cfg.Theme = themeByOption[themeSelect.Selected]
			u.cfg.Language = languageByOption[languageSelect.Selected]
			u.cfg.UseMakepkg = makepkg.Checked
			u.cfg.RemoveTempFiles = cleanup.Checked
			u.cfg.AutoInstallDeps = autoDeps.Checked
			u.cfg.CreateDesktop = createDesktop.Checked
			u.cfg.ValidateDesktop = validateDesktop.Checked
			u.cfg.AutoDetectIcons = detectIcons.Checked
			u.cfg.ShowLogAfterMake = showLog.Checked
			u.cfg.OutputDir = outputDir.Text

			u.conv.SetConfig(u.cfg)
			applyTheme(u.app, u.cfg.Theme)
			if err := config.Save(config.Path(), u.cfg); err != nil {
				u.showError(err)
			}
			if previousLanguage != u.cfg.Language {
				u.tr.SetLanguage(u.cfg.Language)
				u.build()
				if u.current != nil {
					u.populate(u.current)
				}
			}
			u.log.Infof("Settings saved")
		}, u.win)
	form.Resize(fyne.NewSize(560, 460))
	form.Show()
}
