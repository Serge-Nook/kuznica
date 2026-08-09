package ui

import (
	"context"
	"path/filepath"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Serge-Nook/kuznica/internal/converter"
	"github.com/Serge-Nook/kuznica/internal/desktop"
)

// showDesktopDialog asks what to do with an existing entry and then opens
// the editor for the generated Desktop Entry.
func (u *UI) showDesktopDialog(conv *converter.Conversion) {
	existing := desktop.FindExisting(conv.Package.DataDir())
	if len(existing) == 0 {
		u.showDesktopEditor(conv, conv.DesktopEntry)
		return
	}

	options := []string{
		u.tr.T("desktop.use_existing"),
		u.tr.T("desktop.replace"),
		u.tr.T("desktop.create_new"),
		u.tr.T("desktop.edit"),
	}
	choice := widget.NewRadioGroup(options, nil)
	choice.SetSelected(options[0])

	content := container.NewVBox(
		widget.NewLabel(u.tr.T("desktop.exists")),
		widget.NewLabel(filepath.Base(existing[0])),
		choice,
	)
	dialog.NewCustomConfirm(u.tr.T("desktop.title"), u.tr.T("button.save"), u.tr.T("button.cancel"), content,
		func(confirmed bool) {
			if !confirmed {
				return
			}
			switch choice.Selected {
			case options[0]: // use existing
				conv.DesktopPath = existing[0]
				u.log.Infof("Existing desktop entry kept: %s", existing[0])
			case options[1]: // replace
				conv.DesktopPath = existing[0]
				go u.saveDesktop(conv, conv.DesktopEntry)
			case options[2]: // create new
				conv.DesktopPath = filepath.Join(filepath.Dir(existing[0]),
					conv.Spec.PkgName+"-kuznica.desktop")
				go u.saveDesktop(conv, conv.DesktopEntry)
			case options[3]: // open editor
				u.showDesktopEditor(conv, conv.DesktopEntry)
			}
		}, u.win).Show()
}

func (u *UI) showDesktopEditor(conv *converter.Conversion, entry desktop.Entry) {
	name := widget.NewEntry()
	name.SetText(entry.Name)
	comment := widget.NewEntry()
	comment.SetText(entry.Comment)
	execEntry := widget.NewEntry()
	execEntry.SetText(entry.Exec)
	icon := widget.NewEntry()
	icon.SetText(entry.Icon)
	categories := widget.NewEntry()
	categories.SetText(entry.Categories)
	terminal := widget.NewCheck("", nil)
	terminal.SetChecked(entry.Terminal)
	startup := widget.NewCheck("", nil)
	startup.SetChecked(entry.StartupNotify)

	form := widget.NewForm(
		widget.NewFormItem(u.tr.T("desktop.name"), name),
		widget.NewFormItem(u.tr.T("desktop.comment"), comment),
		widget.NewFormItem(u.tr.T("desktop.exec"), execEntry),
		widget.NewFormItem(u.tr.T("desktop.icon"), icon),
		widget.NewFormItem(u.tr.T("desktop.categories"), categories),
		widget.NewFormItem(u.tr.T("desktop.terminal"), terminal),
		widget.NewFormItem(u.tr.T("desktop.startup"), startup),
	)

	editor := dialog.NewCustomConfirm(u.tr.T("desktop.title"), u.tr.T("button.save"), u.tr.T("button.cancel"), form,
		func(save bool) {
			if !save {
				return
			}
			edited := desktop.Entry{
				Type:          "Application",
				Name:          name.Text,
				Comment:       comment.Text,
				Exec:          execEntry.Text,
				Icon:          icon.Text,
				Categories:    categories.Text,
				Terminal:      terminal.Checked,
				StartupNotify: startup.Checked,
			}
			go u.saveDesktop(conv, edited)
		}, u.win)
	editor.Resize(fyne.NewSize(460, 340))
	editor.Show()
}

func (u *UI) saveDesktop(conv *converter.Conversion, entry desktop.Entry) {
	if err := u.conv.SaveDesktopEntry(context.Background(), conv, entry); err != nil {
		u.showError(err)
	}
}
