// Package ui implements the Fyne graphical interface of КУЗНИЦА.
package ui

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	fyneapp "fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/Serge-Nook/kuznica/assets"
	"github.com/Serge-Nook/kuznica/internal/config"
	"github.com/Serge-Nook/kuznica/internal/converter"
	"github.com/Serge-Nook/kuznica/internal/deb"
	"github.com/Serge-Nook/kuznica/internal/i18n"
	"github.com/Serge-Nook/kuznica/internal/installer"
	"github.com/Serge-Nook/kuznica/internal/logger"
	"github.com/Serge-Nook/kuznica/internal/mapping"
)

// Version is the released version of КУЗНИЦА.
const Version = "1.0.3"

// UI owns the main window and all its widgets.
type UI struct {
	app fyne.App
	win fyne.Window

	cfg  config.Config
	log  *logger.Logger
	tr   *i18n.Translator
	conv *converter.Converter

	current *converter.Conversion

	status     *widget.Label
	infoLabels map[string]*widget.Label
	deps       []string
	depsList   *widget.List
	files      []string
	filesList  *widget.List
	journal    *widget.Entry
	progress   *widget.ProgressBarInfinite

	btnOpen    *widget.Button
	btnConvert *widget.Button
	btnBuild   *widget.Button
	btnInstall *widget.Button
	btnFolder  *widget.Button
	btnJournal *widget.Button
}

var infoKeys = []string{
	"field.name", "field.version", "field.architecture", "field.description",
	"field.size", "field.license", "field.maintainer", "field.homepage",
	"field.conflicts", "field.recommends", "field.checksums",
}

// New builds the application window.
func New(cfg config.Config, log *logger.Logger, mappings *mapping.Database) *UI {
	application := fyneapp.NewWithID("ru.sd-on.kuznica")
	application.SetIcon(fyne.NewStaticResource("kuznica.svg", assets.AppIcon))
	applyTheme(application, cfg)

	ui := &UI{
		app:  application,
		cfg:  cfg,
		log:  log,
		tr:   i18n.New(cfg.Language),
		conv: converter.New(cfg, log, mappings),
	}
	ui.win = application.NewWindow(ui.tr.T("app.title"))
	// Fits a 1280x800 desktop with room for panels and window decorations.
	ui.win.Resize(fyne.NewSize(1120, 700))
	ui.build()

	log.Subscribe(func(entry logger.Entry) {
		ui.appendJournal(entry.String())
	})
	return ui
}

// Run shows the window and starts the event loop, optionally opening a
// package passed on the command line.
func (u *UI) Run(initialFile string) {
	if initialFile != "" {
		go u.openPackage(initialFile)
	}
	u.win.ShowAndRun()
}

func (u *UI) build() {
	u.win.SetTitle(u.tr.T("app.title"))
	u.win.SetMainMenu(u.buildMenu())
	u.win.SetContent(u.buildContent())
	u.win.SetOnDropped(func(_ fyne.Position, uris []fyne.URI) {
		if len(uris) == 0 {
			return
		}
		go u.openPackage(uris[0].Path())
	})
	u.refreshButtons()
}

func (u *UI) buildMenu() *fyne.MainMenu {
	file := fyne.NewMenu(u.tr.T("menu.file"),
		fyne.NewMenuItem(u.tr.T("menu.open"), u.showOpenDialog),
		fyne.NewMenuItemSeparator(),
		fyne.NewMenuItem(u.tr.T("menu.quit"), func() { u.app.Quit() }),
	)
	edit := fyne.NewMenu(u.tr.T("menu.edit"),
		fyne.NewMenuItem(u.tr.T("menu.copy_log"), func() {
			u.win.Clipboard().SetContent(u.log.Text())
		}),
		fyne.NewMenuItem(u.tr.T("menu.clear_log"), func() {
			u.log.Clear()
			u.journal.SetText("")
		}),
	)
	settings := fyne.NewMenu(u.tr.T("menu.settings"),
		fyne.NewMenuItem(u.tr.T("menu.preferences"), u.showSettings),
	)
	help := fyne.NewMenu(u.tr.T("menu.help"),
		fyne.NewMenuItem(u.tr.T("menu.about"), u.showAbout),
	)
	return fyne.NewMainMenu(file, edit, settings, help)
}

func (u *UI) buildContent() fyne.CanvasObject {
	title := widget.NewLabelWithStyle(u.tr.T("app.title"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	subtitle := widget.NewLabelWithStyle(u.tr.T("app.subtitle"), fyne.TextAlignLeading, fyne.TextStyle{Italic: true})

	dropHint := widget.NewLabelWithStyle(u.tr.T("drop.hint"), fyne.TextAlignTrailing, fyne.TextStyle{})
	u.btnOpen = widget.NewButtonWithIcon(u.tr.T("button.open"), theme.FolderOpenIcon(), u.showOpenDialog)
	// Single header row keeps the vertical space for the package contents.
	header := container.NewBorder(nil, nil,
		container.NewHBox(title, subtitle),
		container.NewHBox(dropHint, u.btnOpen),
	)

	u.infoLabels = map[string]*widget.Label{}
	infoGrid := container.New(layout.NewFormLayout())
	for _, key := range infoKeys {
		name := widget.NewLabelWithStyle(u.tr.T(key), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
		value := widget.NewLabel("—")
		value.Wrapping = fyne.TextWrapWord
		u.infoLabels[key] = value
		infoGrid.Add(name)
		infoGrid.Add(value)
	}

	u.depsList = widget.NewList(
		func() int { return len(u.deps) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, object fyne.CanvasObject) {
			object.(*widget.Label).SetText(u.deps[id])
		},
	)
	u.filesList = widget.NewList(
		func() int { return len(u.files) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(id widget.ListItemID, object fyne.CanvasObject) {
			object.(*widget.Label).SetText(u.files[id])
		},
	)

	u.journal = widget.NewMultiLineEntry()
	u.journal.Wrapping = fyne.TextWrapOff
	u.journal.SetText(u.log.Text())

	tabs := container.NewAppTabs(
		container.NewTabItem(u.tr.T("section.info"), container.NewVScroll(infoGrid)),
		container.NewTabItem(u.tr.T("section.dependencies"), u.depsList),
		container.NewTabItem(u.tr.T("section.files"), u.filesList),
		container.NewTabItem(u.tr.T("section.log"), u.journal),
	)

	u.btnConvert = widget.NewButtonWithIcon(u.tr.T("button.convert"), theme.MediaReplayIcon(), func() { go u.convert() })
	u.btnBuild = widget.NewButtonWithIcon(u.tr.T("button.make"), theme.StorageIcon(), func() { go u.buildPackage() })
	u.btnInstall = widget.NewButtonWithIcon(u.tr.T("button.install"), theme.DownloadIcon(), func() { go u.install() })
	u.btnFolder = widget.NewButtonWithIcon(u.tr.T("button.open_folder"), theme.FolderIcon(), u.openFolder)
	u.btnJournal = widget.NewButtonWithIcon(u.tr.T("button.show_log"), theme.DocumentIcon(), func() { tabs.SelectIndex(3) })

	actions := container.NewGridWithColumns(5, u.btnConvert, u.btnBuild, u.btnInstall, u.btnFolder, u.btnJournal)
	actions = container.NewPadded(actions)

	u.status = widget.NewLabel(u.tr.T("status.ready"))
	u.progress = widget.NewProgressBarInfinite()
	u.progress.Stop()
	u.progress.Hide()

	top := container.NewVBox(header, widget.NewSeparator())
	bottom := container.NewVBox(widget.NewSeparator(), actions, container.NewBorder(nil, nil, u.status, nil, u.progress))
	return container.NewBorder(top, bottom, nil, nil, tabs)
}

func (u *UI) showOpenDialog() {
	open := dialog.NewFileOpen(func(reader fyne.URIReadCloser, err error) {
		if err != nil {
			u.showError(err)
			return
		}
		if reader == nil {
			return
		}
		path := reader.URI().Path()
		reader.Close()
		go u.openPackage(path)
	}, u.win)
	open.SetFilter(storage.NewExtensionFileFilter([]string{".deb"}))
	open.Resize(fyne.NewSize(760, 480))
	open.Show()
}

func (u *UI) openPackage(path string) {
	u.setBusy(u.tr.T("status.opening"))
	defer u.setIdle()

	conv, err := u.conv.Open(path)
	if err != nil {
		u.showError(err)
		return
	}
	if u.current != nil && u.current.Package != nil {
		u.current.Package.Cleanup()
	}
	u.current = conv
	u.populate(conv)
	u.setStatus(u.tr.T("status.opened") + ": " + filepath.Base(path))
}

func (u *UI) populate(conv *converter.Conversion) {
	pkg := conv.Package
	set := func(key, value string) {
		if value == "" {
			value = "—"
		}
		u.infoLabels[key].SetText(value)
	}
	set("field.name", pkg.Name)
	set("field.version", pkg.Version)
	set("field.architecture", pkg.Architecture)
	set("field.description", pkg.Description)
	set("field.size", fmt.Sprintf("%s (%s)", deb.HumanSize(pkg.FileSize), deb.HumanSize(pkg.InstalledBytes())))
	set("field.license", pkg.License)
	set("field.maintainer", pkg.Maintainer)
	set("field.homepage", pkg.Homepage)
	set("field.conflicts", strings.Join(pkg.Conflicts, ", "))
	set("field.recommends", strings.Join(pkg.Recommends, ", "))
	set("field.checksums", fmt.Sprintf("%d", len(pkg.Checksums)))

	u.deps = nil
	for _, dep := range append(append([]string{}, pkg.PreDepends...), pkg.Depends...) {
		res := u.conv.Mappings().Translate(dep)
		switch {
		case res.Ignored:
			u.deps = append(u.deps, fmt.Sprintf("%s → —", res.Debian))
		default:
			u.deps = append(u.deps, fmt.Sprintf("%s → %s", res.Debian, res.Arch))
		}
	}
	u.depsList.Refresh()

	u.files = make([]string, 0, len(pkg.Files))
	for _, file := range pkg.Files {
		if file.IsDir {
			continue
		}
		u.files = append(u.files, "/"+file.Path)
	}
	u.filesList.Refresh()
	u.refreshButtons()
}

func (u *UI) convert() {
	if u.current == nil {
		u.showMessage(u.tr.T("error.no_package"))
		return
	}
	u.setBusy(u.tr.T("status.converting"))
	defer u.setIdle()

	if err := u.conv.Convert(u.current); err != nil {
		u.showError(err)
		return
	}
	u.setStatus(u.tr.T("status.converted"))
	u.refreshButtons()
	if u.cfg.CreateDesktop && u.current.DesktopPath != "" {
		u.showDesktopDialog(u.current)
	}
}

func (u *UI) buildPackage() {
	if u.current == nil {
		u.showMessage(u.tr.T("error.no_package"))
		return
	}
	if u.current.BuildDir == "" {
		u.showMessage(u.tr.T("error.not_converted"))
		return
	}
	u.setBusy(u.tr.T("status.building"))
	defer u.setIdle()

	if err := u.conv.Build(context.Background(), u.current); err != nil {
		u.showError(err)
		return
	}
	u.setStatus(u.tr.T("status.built"))
	u.refreshButtons()
	if u.cfg.ShowLogAfterMake {
		u.showMessage(u.tr.T("status.built") + ": " + filepath.Base(u.current.ArtifactPath))
	}
}

func (u *UI) install() {
	if u.current == nil || u.current.ArtifactPath == "" {
		u.showMessage(u.tr.T("error.not_built"))
		return
	}
	u.setBusy(u.tr.T("status.installing"))
	defer u.setIdle()

	if err := u.conv.Install(context.Background(), u.current); err != nil {
		u.showError(err)
		return
	}
	u.setStatus(u.tr.T("status.installed"))
}

func (u *UI) openFolder() {
	dir := u.cfg.OutputDir
	if u.current != nil && u.current.BuildDir != "" {
		dir = u.current.BuildDir
	}
	if err := installer.OpenFolder(context.Background(), dir); err != nil {
		u.showError(err)
	}
}

func (u *UI) refreshButtons() {
	hasPackage := u.current != nil && u.current.Package != nil
	converted := hasPackage && u.current.BuildDir != ""
	built := converted && u.current.ArtifactPath != ""

	toggle := func(button *widget.Button, enabled bool) {
		if button == nil {
			return
		}
		if enabled {
			button.Enable()
			return
		}
		button.Disable()
	}
	toggle(u.btnConvert, hasPackage)
	toggle(u.btnBuild, converted)
	toggle(u.btnInstall, built)
	toggle(u.btnFolder, converted)
}

func (u *UI) appendJournal(line string) {
	if u.journal == nil {
		return
	}
	text := u.journal.Text
	if text != "" {
		text += "\n"
	}
	u.journal.SetText(text + line)
	u.journal.CursorRow = strings.Count(u.journal.Text, "\n")
}

func (u *UI) setStatus(text string) {
	if u.status != nil {
		u.status.SetText(text)
	}
}

func (u *UI) setBusy(text string) {
	u.setStatus(text)
	if u.progress != nil {
		u.progress.Show()
		u.progress.Start()
	}
}

func (u *UI) setIdle() {
	if u.progress != nil {
		u.progress.Stop()
		u.progress.Hide()
	}
}

func (u *UI) showError(err error) {
	u.log.Errorf("%v", err)
	dialog.ShowError(u.describeError(err), u.win)
}

// describeError replaces the toolchain errors with a translated hint that
// tells the user which Arch Linux package to install.
func (u *UI) describeError(err error) error {
	switch {
	case errors.Is(err, installer.ErrFakerootMissing), errors.Is(err, installer.ErrMakepkgMissing):
		return errors.New(u.tr.T("error.base_devel"))
	case errors.Is(err, installer.ErrPacmanMissing):
		return errors.New(u.tr.T("error.pacman"))
	case errors.Is(err, installer.ErrReadOnlyRoot):
		return errors.New(u.tr.T("error.readonly"))
	default:
		return err
	}
}

func (u *UI) showMessage(text string) {
	dialog.ShowInformation(u.tr.T("app.title"), text, u.win)
}
