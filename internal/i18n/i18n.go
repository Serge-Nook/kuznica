// Package i18n provides the Russian and English translations of the
// КУЗНИЦА user interface.
package i18n

import "sync"

// Language codes supported by the application.
const (
	Russian = "ru"
	English = "en"
)

// Languages lists the supported language codes in menu order.
var Languages = []string{Russian, English}

// LanguageNames maps a language code to its native name.
var LanguageNames = map[string]string{
	Russian: "Русский",
	English: "English",
}

var translations = map[string]map[string]string{
	Russian: {
		"app.title":              "КУЗНИЦА",
		"app.subtitle":           "Конвертер пакетов Debian → Arch Linux",
		"menu.file":              "Файл",
		"menu.edit":              "Правка",
		"menu.settings":          "Настройки",
		"menu.help":              "Справка",
		"menu.open":              "Открыть…",
		"menu.quit":              "Выход",
		"menu.copy_log":          "Копировать журнал",
		"menu.clear_log":         "Очистить журнал",
		"menu.preferences":       "Параметры…",
		"menu.about":             "О программе",
		"drop.hint":              "Перетащите .deb сюда",
		"drop.or":                "или",
		"button.open":            "Открыть",
		"button.convert":         "Конвертировать",
		"button.make":            "Создать пакет",
		"button.install":         "Установить",
		"button.open_folder":     "Открыть папку",
		"button.show_log":        "Просмотреть журнал",
		"button.save":            "Сохранить",
		"button.cancel":          "Отмена",
		"button.close":           "Закрыть",
		"section.info":           "Информация",
		"section.dependencies":   "Зависимости",
		"section.files":          "Файлы",
		"section.log":            "Журнал",
		"field.name":             "Название",
		"field.version":          "Версия",
		"field.architecture":     "Архитектура",
		"field.description":      "Описание",
		"field.size":             "Размер",
		"field.license":          "Лицензия",
		"field.maintainer":       "Автор",
		"field.homepage":         "Сайт",
		"field.conflicts":        "Конфликты",
		"field.recommends":       "Рекомендуемые",
		"field.checksums":        "Контрольные суммы",
		"settings.title":         "Настройки",
		"settings.general":       "Общие",
		"settings.work":          "Работа",
		"settings.theme":         "Тема",
		"settings.theme.dark":    "Тёмная тема",
		"settings.theme.light":   "Светлая тема",
		"settings.theme.sys":     "Системная тема",
		"settings.language":      "Язык",
		"settings.scale":         "Масштаб интерфейса",
		"settings.scale.compact": "Компактный (1280×800)",
		"settings.scale.normal":  "Обычный",
		"settings.scale.large":   "Крупный",
		"settings.makepkg":       "Использовать makepkg (иначе встроенный сборщик)",
		"settings.cleanup":       "Удалять временные файлы",
		"settings.autodeps":      "Автоматически устанавливать зависимости",
		"settings.desktop":       "Создавать .desktop",
		"settings.validate":      "Проверять .desktop",
		"settings.icons":         "Искать иконки автоматически",
		"settings.showlog":       "Показывать журнал после сборки",
		"settings.output":        "Каталог сборки",
		"settings.steamos":       "SteamOS",
		"settings.gamemode":      "Адаптировать для игрового режима SteamOS",
		"settings.gamemode.hint": "Выключено по умолчанию. Если включить, то сразу после сборки программа" +
			" устанавливается в ~/Applications (без root, не стирается обновлением SteamOS)" +
			" и добавляется в библиотеку Steam как сторонняя игра — только так её можно запустить в" +
			" игровом режиме. Без этой настройки адаптация выполняется только кнопкой «Игровой режим».",
		"button.gamemode":   "Игровой режим",
		"gamemode.title":    "Адаптация для игрового режима",
		"gamemode.prefix":   "Программа установлена в:",
		"gamemode.launcher": "Запускающий скрипт:",
		"gamemode.shortcut": "Ярлык добавлен в профили Steam:",
		"gamemode.howto": "Как запустить: перейдите в игровой режим, откройте «Библиотека» → «Не Steam»" +
			" и выберите программу. Ярлык также появится в меню рабочего стола.",
		"gamemode.restart": "Steam запущен: он перезаписывает список ярлыков при выходе." +
			" Перезапустите Steam (или переключитесь в игровой режим), чтобы увидеть ярлык.",
		"gamemode.no_steam":    "Steam не найден: нужен профиль в ~/.steam или ~/.local/share/Steam",
		"gamemode.no_exec":     "В пакете не найден исполняемый файл для запуска",
		"status.gamemode":      "Адаптация для игрового режима…",
		"status.gamemode_done": "Программа добавлена в библиотеку Steam",
		"desktop.title":        "Файл .desktop",
		"desktop.name":         "Name",
		"desktop.comment":      "Comment",
		"desktop.exec":         "Exec",
		"desktop.icon":         "Icon",
		"desktop.categories":   "Categories",
		"desktop.terminal":     "Terminal",
		"desktop.startup":      "StartupNotify",
		"desktop.exists":       "Файл .desktop уже существует. Что сделать?",
		"desktop.use_existing": "Использовать существующий",
		"desktop.replace":      "Заменить",
		"desktop.create_new":   "Создать новый",
		"desktop.edit":         "Открыть редактор",
		"about.version":        "Версия",
		"about.author":         "Автор",
		"about.site":           "Сайт",
		"about.donate":         "Пожертвования",
		"about.language":       "Разработано на языке Go",
		"about.platform":       "Платформа",
		"status.ready":         "Готово к работе",
		"status.opening":       "Открытие пакета…",
		"status.opened":        "Пакет открыт",
		"status.converting":    "Конвертация…",
		"status.converted":     "Конвертация завершена",
		"status.building":      "Сборка пакета…",
		"status.built":         "Пакет собран",
		"status.installing":    "Установка пакета…",
		"status.installed":     "Пакет установлен",
		"error.title":          "Ошибка",
		"error.no_package":     "Сначала откройте пакет .deb",
		"error.not_converted":  "Сначала выполните конвертацию",
		"error.not_built":      "Сначала соберите пакет",
		"error.base_devel":     "Для сборки нужны makepkg и fakeroot.\nУстановите их: sudo pacman -S --needed base-devel",
		"error.pacman":         "pacman не найден: программа работает только на Arch Linux",
		"error.readonly":       "Корневая файловая система только для чтения (SteamOS).\nСделайте её записываемой: sudo steamos-readonly disable",
	},
	English: {
		"app.title":              "KUZNICA",
		"app.subtitle":           "Debian → Arch Linux package converter",
		"menu.file":              "File",
		"menu.edit":              "Edit",
		"menu.settings":          "Settings",
		"menu.help":              "Help",
		"menu.open":              "Open…",
		"menu.quit":              "Quit",
		"menu.copy_log":          "Copy journal",
		"menu.clear_log":         "Clear journal",
		"menu.preferences":       "Preferences…",
		"menu.about":             "About",
		"drop.hint":              "Drop a .deb file here",
		"drop.or":                "or",
		"button.open":            "Open",
		"button.convert":         "Convert",
		"button.make":            "Build package",
		"button.install":         "Install",
		"button.open_folder":     "Open folder",
		"button.show_log":        "Show journal",
		"button.save":            "Save",
		"button.cancel":          "Cancel",
		"button.close":           "Close",
		"section.info":           "Information",
		"section.dependencies":   "Dependencies",
		"section.files":          "Files",
		"section.log":            "Journal",
		"field.name":             "Name",
		"field.version":          "Version",
		"field.architecture":     "Architecture",
		"field.description":      "Description",
		"field.size":             "Size",
		"field.license":          "License",
		"field.maintainer":       "Maintainer",
		"field.homepage":         "Homepage",
		"field.conflicts":        "Conflicts",
		"field.recommends":       "Recommends",
		"field.checksums":        "Checksums",
		"settings.title":         "Settings",
		"settings.general":       "General",
		"settings.work":          "Workflow",
		"settings.theme":         "Theme",
		"settings.theme.dark":    "Dark theme",
		"settings.theme.light":   "Light theme",
		"settings.theme.sys":     "System theme",
		"settings.language":      "Language",
		"settings.scale":         "Interface scale",
		"settings.scale.compact": "Compact (1280×800)",
		"settings.scale.normal":  "Normal",
		"settings.scale.large":   "Large",
		"settings.makepkg":       "Use makepkg (otherwise the built-in packager)",
		"settings.cleanup":       "Remove temporary files",
		"settings.autodeps":      "Install dependencies automatically",
		"settings.desktop":       "Create .desktop entry",
		"settings.validate":      "Validate .desktop entry",
		"settings.icons":         "Detect icons automatically",
		"settings.showlog":       "Show journal after build",
		"settings.output":        "Build directory",
		"settings.steamos":       "SteamOS",
		"settings.gamemode":      "Adapt for the SteamOS game mode",
		"settings.gamemode.hint": "Disabled by default. When enabled, the program is installed into ~/Applications" +
			" (no root, survives SteamOS updates) right after the build and added to the Steam library as a" +
			" non-Steam game, which is the only way to start it in game mode. Otherwise the adaptation runs" +
			" only from the \"Game mode\" button.",
		"button.gamemode":   "Game mode",
		"gamemode.title":    "Game mode adaptation",
		"gamemode.prefix":   "The program is installed in:",
		"gamemode.launcher": "Launcher script:",
		"gamemode.shortcut": "Shortcut added to the Steam profiles:",
		"gamemode.howto": "How to start it: switch to game mode, open Library → Non-Steam" +
			" and pick the program. The shortcut also appears in the desktop menu.",
		"gamemode.restart": "Steam is running: it rewrites the shortcut list on exit." +
			" Restart Steam (or switch to game mode) to see the shortcut.",
		"gamemode.no_steam":    "Steam was not found: a profile in ~/.steam or ~/.local/share/Steam is required",
		"gamemode.no_exec":     "No executable to start was found in the package",
		"status.gamemode":      "Adapting for the game mode…",
		"status.gamemode_done": "The program was added to the Steam library",
		"desktop.title":        "Desktop entry",
		"desktop.name":         "Name",
		"desktop.comment":      "Comment",
		"desktop.exec":         "Exec",
		"desktop.icon":         "Icon",
		"desktop.categories":   "Categories",
		"desktop.terminal":     "Terminal",
		"desktop.startup":      "StartupNotify",
		"desktop.exists":       "A .desktop entry already exists. What should be done?",
		"desktop.use_existing": "Use existing",
		"desktop.replace":      "Replace",
		"desktop.create_new":   "Create new",
		"desktop.edit":         "Open editor",
		"about.version":        "Version",
		"about.author":         "Author",
		"about.site":           "Website",
		"about.donate":         "Donations",
		"about.language":       "Written in Go",
		"about.platform":       "Platform",
		"status.ready":         "Ready",
		"status.opening":       "Opening package…",
		"status.opened":        "Package opened",
		"status.converting":    "Converting…",
		"status.converted":     "Conversion finished",
		"status.building":      "Building package…",
		"status.built":         "Package built",
		"status.installing":    "Installing package…",
		"status.installed":     "Package installed",
		"error.title":          "Error",
		"error.no_package":     "Open a .deb package first",
		"error.not_converted":  "Run the conversion first",
		"error.not_built":      "Build the package first",
		"error.base_devel":     "makepkg and fakeroot are required to build.\nInstall them: sudo pacman -S --needed base-devel",
		"error.pacman":         "pacman was not found: this program only works on Arch Linux",
		"error.readonly":       "The root filesystem is read-only (SteamOS).\nMake it writable: sudo steamos-readonly disable",
	},
}

// Translator resolves message keys for the currently selected language.
type Translator struct {
	mu       sync.RWMutex
	language string
}

// New returns a translator for the given language code.
func New(language string) *Translator {
	t := &Translator{}
	t.SetLanguage(language)
	return t
}

// SetLanguage switches the active language, ignoring unsupported codes.
func (t *Translator) SetLanguage(language string) {
	if _, ok := translations[language]; !ok {
		language = Russian
	}
	t.mu.Lock()
	t.language = language
	t.mu.Unlock()
}

// Language returns the active language code.
func (t *Translator) Language() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.language
}

// T returns the translation of key, falling back to English and then to the
// key itself so that a missing string is always visible.
func (t *Translator) T(key string) string {
	t.mu.RLock()
	language := t.language
	t.mu.RUnlock()

	if value, ok := translations[language][key]; ok {
		return value
	}
	if value, ok := translations[English][key]; ok {
		return value
	}
	return key
}
