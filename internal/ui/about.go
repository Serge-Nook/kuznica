package ui

import (
	"net/url"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/Serge-Nook/kuznica/assets"
)

// Project constants shown in the About dialog.
const (
	Author      = "Горшков Сергей Владимирович"
	WebsiteURL  = "https://sd-on.ru"
	DonationURL = "https://sd-on.ru/donate/"
	Platform    = "Arch Linux"
)

func (u *UI) showAbout() {
	icon := canvas.NewImageFromResource(fyne.NewStaticResource("kuznica.svg", assets.AppIcon))
	icon.FillMode = canvas.ImageFillContain
	icon.SetMinSize(fyne.NewSize(64, 64))

	site, _ := url.Parse(WebsiteURL)
	donate, _ := url.Parse(DonationURL)

	content := container.NewVBox(
		container.NewCenter(icon),
		widget.NewLabelWithStyle(u.tr.T("app.title"), fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		widget.NewLabelWithStyle(u.tr.T("about.version")+": "+Version, fyne.TextAlignCenter, fyne.TextStyle{}),
		widget.NewSeparator(),
		widget.NewLabel(u.tr.T("about.author")+": "+Author),
		container.NewHBox(widget.NewLabel(u.tr.T("about.site")+":"), widget.NewHyperlink(WebsiteURL, site)),
		container.NewHBox(widget.NewLabel(u.tr.T("about.donate")+":"), widget.NewHyperlink(DonationURL, donate)),
		widget.NewSeparator(),
		widget.NewLabel(u.tr.T("about.language")),
		widget.NewLabel(u.tr.T("about.platform")+": "+Platform),
	)

	about := dialog.NewCustom(u.tr.T("menu.about"), u.tr.T("button.close"), content, u.win)
	about.Resize(fyne.NewSize(380, 340))
	about.Show()
}
