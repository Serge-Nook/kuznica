# КУЗНИЦА / KUZNICA

Графическая утилита для Arch Linux, превращающая пакеты Debian (`.deb`) в
нативные пакеты Arch Linux (`.pkg.tar.zst`).

*A Fyne GUI tool for Arch Linux that converts Debian `.deb` packages into
native `.pkg.tar.zst` packages.*

* Версия / Version: **1.0.3**
* Автор / Author: Горшков Сергей Владимирович
* Сайт / Website: <https://sd-on.ru>
* Пожертвования / Donations: <https://sd-on.ru/donate/>

## Возможности

* открытие `.deb` через Drag & Drop, кнопку «Открыть» или аргумент командной строки;
* распаковка `debian-binary`, `control.tar` и `data.tar` (`gz`, `xz`, `zst`, `bz2`, без сжатия);
* анализ метаданных: название, версия, описание, архитектура, размер, лицензия,
  автор, сайт, список файлов, зависимости, конфликты, рекомендации, контрольные суммы;
* конвертация зависимостей Debian → Arch по встроенной базе `assets/mappings.json`,
  которую пользователь может дополнять своим файлом;
* генерация `PKGBUILD`, `.SRCINFO` и `.INSTALL` (из maintainer-скриптов Debian);
* сборка через `makepkg` и установка через `sudo`/`pkexec` + `pacman -U`;
* автоматическое создание и проверка `.desktop` (`desktop-file-validate`);
* журнал всех действий на экране и в файле;
* русский и английский интерфейс, светлая/тёмная/системная тема.

## Установка и запуск

```bash
# зависимости: go >= 1.22, base-devel, а для сборки GUI - libxkbcommon, mesa
git clone https://github.com/Serge-Nook/kuznica.git
cd kuznica
make build          # бинарник в build/kuznica
./build/kuznica     # или ./build/kuznica package.deb
```

AppImage (не требует установки):

```bash
# скачайте KUZNICA-1.0.3-x86_64.AppImage со страницы релизов
chmod +x KUZNICA-1.0.3-x86_64.AppImage
./KUZNICA-1.0.3-x86_64.AppImage program.deb
```

Собрать AppImage самостоятельно: `./packaging/build-appimage.sh` (результат в `dist/`).

Установка самой КУЗНИЦЫ как пакета Arch Linux:

```bash
cd packaging && makepkg -si
```

## Использование

1. Перетащите `.deb` в окно или нажмите **Открыть**.
2. Проверьте вкладки «Информация», «Зависимости», «Файлы».
3. Нажмите **Конвертировать** — будут созданы `PKGBUILD`, `.SRCINFO`, `.INSTALL`
   в каталоге сборки (по умолчанию `~/kuznica/<pkgname>`).
4. Нажмите **Создать пакет** — запускается `makepkg`, результат `*.pkg.tar.zst`.
5. Нажмите **Установить** — `pacman -U` с запросом прав через `pkexec`/`sudo`.

Подробнее: [docs/USAGE.md](docs/USAGE.md), архитектура: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## База соответствия зависимостей

Встроенная база лежит в [`assets/mappings.json`](assets/mappings.json):

| Debian | Arch |
| --- | --- |
| `libc6` | `glibc` |
| `python3` | `python` |
| `libgtk-3-0` | `gtk3` |
| `libssl3` | `openssl` |
| `libnotify4` | `libnotify` |

Пользовательские правила читаются из `~/.config/kuznica/mappings.json` и
переопределяют встроенные. Неизвестные пакеты приводятся эвристикой
(`libfoo7` → `libfoo`, `foo-dev` → `foo`), пакеты из списка `ignored`
(`dpkg`, `debconf`, …) отбрасываются.

## Безопасность

* пакет распаковывается только во временный каталог (`os.MkdirTemp`);
* каждый элемент архива проверяется на выход за пределы каталога (path traversal);
* контрольные суммы `md5sums` пересчитываются после распаковки;
* конвертация и `makepkg` выполняются без прав root;
* `sudo`/`pkexec` запрашивается только на этапе установки.

## Разработка

```bash
make test     # go test ./...
make lint     # go vet + gofmt -l
make build    # сборка бинарника
```

## Лицензия

MIT — см. [LICENSE](LICENSE).
