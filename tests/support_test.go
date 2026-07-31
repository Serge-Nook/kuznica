package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Serge-Nook/kuznica/internal/config"
	"github.com/Serge-Nook/kuznica/internal/i18n"
	"github.com/Serge-Nook/kuznica/internal/logger"
)

func TestConfigRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load missing file: %v", err)
	}
	if loaded.Language != "ru" || !loaded.CreateDesktop {
		t.Errorf("unexpected defaults: %+v", loaded)
	}

	loaded.Language = "en"
	loaded.Theme = config.ThemeDark
	loaded.AutoInstallDeps = true
	if err := config.Save(path, loaded); err != nil {
		t.Fatalf("save: %v", err)
	}

	reloaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if reloaded.Language != "en" || reloaded.Theme != config.ThemeDark || !reloaded.AutoInstallDeps {
		t.Errorf("settings not persisted: %+v", reloaded)
	}
}

func TestConfigNormalizesUnknownValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"theme": "neon", "language": "fr", "output_dir": ""}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	loaded, err := config.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if loaded.Theme != config.ThemeSystem || loaded.Language != "ru" || loaded.OutputDir == "" {
		t.Errorf("values not normalised: %+v", loaded)
	}
	if loaded.UIScale != config.ScaleCompact {
		t.Errorf("scale not normalised: %v", loaded.UIScale)
	}
}

func TestConfigClampsUIScale(t *testing.T) {
	if got := (config.Config{UIScale: 0}).NormalizedUIScale(); got != config.ScaleCompact {
		t.Errorf("zero scale = %v, want %v", got, config.ScaleCompact)
	}
	if got := (config.Config{UIScale: 5}).NormalizedUIScale(); got != config.ScaleLarge {
		t.Errorf("huge scale = %v, want %v", got, config.ScaleLarge)
	}
	if got := (config.Config{UIScale: config.ScaleNormal}).NormalizedUIScale(); got != config.ScaleNormal {
		t.Errorf("normal scale = %v, want %v", got, config.ScaleNormal)
	}
}

func TestLoggerRecordsAndNotifies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "kuznica.log")
	log, err := logger.NewWithFile(path)
	if err != nil {
		t.Fatalf("logger: %v", err)
	}
	defer log.Close()

	var seen []string
	log.Subscribe(func(entry logger.Entry) { seen = append(seen, entry.Message) })

	log.Infof("opening %s", "hello.deb")
	log.Errorf("boom")

	if len(seen) != 2 || seen[0] != "opening hello.deb" {
		t.Fatalf("observer entries = %v", seen)
	}
	if entries := log.Entries(); len(entries) != 2 || entries[1].Level != logger.LevelError {
		t.Fatalf("entries = %+v", entries)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read log file: %v", err)
	}
	if !strings.Contains(string(data), "opening hello.deb") {
		t.Errorf("journal file misses the entry:\n%s", data)
	}

	log.Clear()
	if len(log.Entries()) != 0 {
		t.Error("Clear did not empty the journal")
	}
}

func TestTranslatorFallsBack(t *testing.T) {
	tr := i18n.New("ru")
	if got := tr.T("button.install"); got != "Установить" {
		t.Errorf("russian translation = %q", got)
	}
	tr.SetLanguage("en")
	if got := tr.T("button.install"); got != "Install" {
		t.Errorf("english translation = %q", got)
	}
	tr.SetLanguage("fr")
	if tr.Language() != "ru" {
		t.Errorf("unsupported language accepted: %q", tr.Language())
	}
	if got := tr.T("missing.key"); got != "missing.key" {
		t.Errorf("missing key = %q", got)
	}
}

func TestEveryRussianKeyHasEnglishTranslation(t *testing.T) {
	ru := i18n.New("ru")
	en := i18n.New("en")
	for _, key := range []string{
		"app.title", "menu.file", "button.convert", "section.files",
		"settings.title", "desktop.title", "about.version", "status.ready",
	} {
		if ru.T(key) == key || en.T(key) == key {
			t.Errorf("key %q is not translated in both languages", key)
		}
	}
}
