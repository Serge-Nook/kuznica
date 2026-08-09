# Архитектура КУЗНИЦЫ

```
cmd/kuznica/        точка входа, разбор аргументов, инициализация логгера и настроек
internal/deb/       чтение ar-контейнера, распаковка control.tar/data.tar, метаданные
internal/mapping/   база соответствия зависимостей Debian ↔ Arch
internal/pkgbuild/  генерация PKGBUILD, .SRCINFO, .INSTALL
internal/desktop/   поиск, генерация, редактирование и проверка .desktop
internal/installer/ запуск makepkg и pacman, проверка места и наличия инструментов
internal/packager/  встроенная сборка .pkg.tar.zst без makepkg и fakeroot
internal/steam/     адаптация для игрового режима SteamOS, чтение/запись shortcuts.vdf
internal/converter/ оркестрация всего конвейера
internal/logger/    журнал (память + файл + подписчики)
internal/config/    пользовательские настройки (~/.config/kuznica/config.json)
internal/i18n/      переводы интерфейса (ru, en)
internal/ui/        графический интерфейс на Fyne v2
assets/             mappings.json, иконки (встраиваются через go:embed)
tests/              интеграционные и модульные тесты на сгенерированных .deb
docs/               документация
```

## Конвейер конвертации

1. **Открытие** (`converter.Open` → `deb.Open`)
   * `ar` контейнер читается потоково, проверяется `debian-binary` (формат `2.x`);
   * `control.tar.*` и `data.tar.*` распаковываются во временный каталог
     `/tmp/kuznica-*/{control,data}`;
   * каждый путь проверяется функцией `safeJoin` (защита от path traversal);
   * из `control` читаются метаданные, из `md5sums` — контрольные суммы,
     из `postinst`/`preinst`/`prerm`/`postrm` — maintainer-скрипты;
   * контрольные суммы пересчитываются и сравниваются.

2. **Конвертация** (`converter.Convert`)
   * зависимости `Depends`/`Pre-Depends` переводятся через `mapping.Database`;
   * версия Debian `1:2.10-3` превращается в `pkgver=2.10`, эпоха и ревизия
     отбрасываются, а ограничения версий переносятся как `glibc>=2.34`;
   * формируется `pkgbuild.Spec`, из него рендерятся `PKGBUILD`, `.SRCINFO`
     и, при наличии maintainer-скриптов, `<pkgname>.install`;
   * распакованный payload подключается к каталогу сборки симлинком `payload`,
     функция `package()` копирует его в `$pkgdir`;
   * при включённой настройке готовится `.desktop`.

3. **Сборка** (`converter.Build` → `installer.Makepkg`)
   * `makepkg --force --noconfirm --nodeps` с `PKGDEST` = каталог сборки;
   * вывод построчно попадает в журнал;
   * результат — `*.pkg.tar.zst`.

4. **Установка** (`converter.Install` → `installer.Install`)
   * `pkexec`/`sudo` + `pacman -U --noconfirm`;
   * root запрашивается только здесь.

5. **Игровой режим SteamOS** (`converter.AdaptGameMode` → `steam.Adapt`)
   * payload копируется в `~/Applications/<pkgname>` (без root, выживает при
     обновлении SteamOS), символические ссылки сохраняются;
   * генерируется `kuznica-launch.sh` с `LD_LIBRARY_PATH`, `XDG_DATA_DIRS`,
     `PATH` и `GSETTINGS_SCHEMA_DIR`, нацеленными на префикс;
   * `shortcuts.vdf` каждого профиля Steam читается своим парсером бинарного
     VDF (типы `0x00`/`0x01`/`0x02`, терминатор `0x08`), запись для программы
     добавляется или обновляется, остальные ярлыки сохраняются без изменений,
     файл перезаписывается атомарно с резервной копией `*.kuznica.bak`;
   * `appid` считается как `crc32("Exe" + AppName) | 0x80000000` — так же, как в
     самом Steam для сторонних игр.

## Потоки и журнал

Длительные операции (открытие, конвертация, сборка, установка) выполняются
в отдельных горутинах, интерфейс показывает индикатор прогресса. Логгер
рассылает записи подписчикам, интерфейс дописывает их во вкладку «Журнал»,
одновременно всё пишется в `$XDG_CACHE_HOME/kuznica/kuznica.log`.

## Обработка ошибок

Типизированные ошибки позволяют показывать понятные сообщения:

| Ошибка | Причина |
| --- | --- |
| `deb.ErrNotDebPackage` | файл не является пакетом Debian или формат не `2.x` |
| `deb.ErrNoControlTar` | в пакете нет `control.tar` |
| `deb.ErrNoDataTar` | в пакете нет `data.tar` |
| `deb.ErrPathTraversal` | элемент архива пытается выйти за пределы каталога |
| `installer.ErrMakepkgMissing` | не установлен `makepkg` (`base-devel`) |
| `installer.ErrPacmanMissing` | не установлен `pacman` |
| `installer.ErrNotEnoughSpace` | недостаточно места на диске |
| `installer.ErrNoElevation` | нет `pkexec` и `sudo` |
| `desktop.ErrValidatorMissing` | не установлен `desktop-file-validate` |
| `installer.ErrReadOnlyRoot` | база pacman на только-читаемой ФС (SteamOS) |
| `steam.ErrNoSteam` | не найден профиль Steam для добавления ярлыка |
| `steam.ErrNoExecutable` | в payload нет исполняемого файла для запуска |
