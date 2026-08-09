package tests

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Serge-Nook/kuznica/internal/steam"
)

// steamHome creates a home directory with a Steam profile.
func steamHome(t *testing.T) (home string, config string) {
	t.Helper()
	home = t.TempDir()
	config = filepath.Join(home, ".local", "share", "Steam", "userdata", "123456", "config")
	if err := os.MkdirAll(config, 0o755); err != nil {
		t.Fatalf("create profile: %v", err)
	}
	return home, config
}

// samplePayload builds a minimal Debian payload tree.
func samplePayload(t *testing.T) string {
	t.Helper()
	payload := t.TempDir()
	binary := filepath.Join(payload, "usr", "bin")
	icons := filepath.Join(payload, "usr", "share", "icons", "hicolor", "48x48", "apps")
	for _, dir := range []string{binary, icons} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	if err := os.WriteFile(filepath.Join(binary, "lolka"), []byte("#!/bin/sh\necho lolka\n"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	if err := os.WriteFile(filepath.Join(icons, "lolka.png"), []byte("png"), 0o644); err != nil {
		t.Fatalf("write icon: %v", err)
	}
	if err := os.Symlink("lolka", filepath.Join(binary, "lolka-link")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	return payload
}

func TestVDFRoundTrip(t *testing.T) {
	root := steam.NewMap()
	shortcuts := steam.NewMap()
	entry := steam.NewMap()
	entry.Set("appid", int32(-1234567))
	entry.Set("AppName", "Лолка")
	entry.Set("Exe", `"/home/deck/Applications/lolka/kuznica-launch.sh"`)
	tags := steam.NewMap()
	tags.Set("0", "KUZNICA")
	entry.Set("tags", tags)
	shortcuts.Set("0", entry)
	root.Set("shortcuts", shortcuts)

	var buffer bytes.Buffer
	if err := steam.WriteVDF(&buffer, root); err != nil {
		t.Fatalf("write: %v", err)
	}

	parsed, err := steam.ReadVDF(bytes.NewReader(buffer.Bytes()))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	list, ok := parsed.Child("shortcuts")
	if !ok {
		t.Fatal("shortcuts section missing")
	}
	first, ok := list.Child("0")
	if !ok {
		t.Fatal("shortcut 0 missing")
	}
	if name := first.String("AppName"); name != "Лолка" {
		t.Errorf("AppName = %q", name)
	}
	if id, _ := first.Get("appid"); id != int32(-1234567) {
		t.Errorf("appid = %v", id)
	}
	if tags, ok := first.Child("tags"); !ok || tags.String("0") != "KUZNICA" {
		t.Error("tags were not preserved")
	}
}

func TestAddShortcutKeepsExistingEntries(t *testing.T) {
	_, config := steamHome(t)
	user := steam.User{AccountID: "123456", ConfigDir: config}

	existing := steam.NewMap()
	list := steam.NewMap()
	other := steam.NewMap()
	other.Set("appid", int32(-42))
	other.Set("AppName", "Other game")
	other.Set("Exe", `"/usr/bin/other"`)
	list.Set("0", other)
	existing.Set("shortcuts", list)

	file, err := os.Create(user.ShortcutsPath())
	if err != nil {
		t.Fatalf("create shortcuts.vdf: %v", err)
	}
	if err := steam.WriteVDF(file, existing); err != nil {
		t.Fatalf("write shortcuts.vdf: %v", err)
	}
	file.Close()

	shortcut := steam.Shortcut{
		Name:     "Лолка",
		Exe:      "/home/deck/Applications/lolka/kuznica-launch.sh",
		StartDir: "/home/deck/Applications/lolka",
	}
	appID, err := steam.AddShortcut(user, shortcut)
	if err != nil {
		t.Fatalf("add: %v", err)
	}
	if appID != steam.AppID(shortcut.Exe, shortcut.Name) {
		t.Errorf("unexpected app id %d", appID)
	}
	if _, err := os.Stat(user.ShortcutsPath() + ".kuznica.bak"); err != nil {
		t.Errorf("backup missing: %v", err)
	}

	// A second call must update the entry instead of duplicating it.
	if _, err := steam.AddShortcut(user, shortcut); err != nil {
		t.Fatalf("re-add: %v", err)
	}

	data, err := os.ReadFile(user.ShortcutsPath())
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	parsed, err := steam.ReadVDF(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	saved, _ := parsed.Child("shortcuts")
	if saved.Len() != 2 {
		t.Fatalf("expected 2 shortcuts, got %d", saved.Len())
	}
	if entry, ok := saved.Child("0"); !ok || entry.String("AppName") != "Other game" {
		t.Error("the existing shortcut was lost")
	}
	entry, ok := saved.Child("1")
	if !ok {
		t.Fatal("new shortcut missing")
	}
	if entry.String("Exe") != `"`+shortcut.Exe+`"` {
		t.Errorf("Exe = %q", entry.String("Exe"))
	}

	removed, err := steam.RemoveShortcut(user, shortcut)
	if err != nil || !removed {
		t.Fatalf("remove: %v (removed=%v)", err, removed)
	}
}

func TestAdaptInstallsPrefixAndShortcut(t *testing.T) {
	home, config := steamHome(t)
	payload := samplePayload(t)

	result, err := steam.Adapt(steam.Request{
		Home:        home,
		PackageName: "lolka",
		Name:        "Лолка",
		Comment:     "Тестовая программа",
		Categories:  "Utility;",
		PayloadDir:  payload,
		Exec:        "/usr/bin/lolka",
		IconPath:    filepath.Join(payload, "usr", "share", "icons", "hicolor", "48x48", "apps", "lolka.png"),
	})
	if err != nil {
		t.Fatalf("adapt: %v", err)
	}

	prefix := filepath.Join(home, "Applications", "lolka")
	if result.Prefix != prefix {
		t.Errorf("prefix = %q", result.Prefix)
	}
	if info, err := os.Stat(filepath.Join(prefix, "usr", "bin", "lolka")); err != nil {
		t.Fatalf("payload not deployed: %v", err)
	} else if info.Mode().Perm()&0o111 == 0 {
		t.Error("the executable is not executable")
	}
	if target, err := os.Readlink(filepath.Join(prefix, "usr", "bin", "lolka-link")); err != nil || target != "lolka" {
		t.Errorf("symlink not deployed: %q (%v)", target, err)
	}

	launcher, err := os.ReadFile(result.Launcher)
	if err != nil {
		t.Fatalf("launcher missing: %v", err)
	}
	script := string(launcher)
	for _, want := range []string{"LD_LIBRARY_PATH", "XDG_DATA_DIRS", `exec "$PREFIX/usr/bin/lolka"`} {
		if !strings.Contains(script, want) {
			t.Errorf("launcher does not contain %q:\n%s", want, script)
		}
	}

	entry, err := os.ReadFile(result.Desktop)
	if err != nil {
		t.Fatalf("desktop entry missing: %v", err)
	}
	if !strings.Contains(string(entry), "Exec="+result.Launcher) {
		t.Errorf("unexpected desktop entry:\n%s", entry)
	}

	data, err := os.ReadFile(filepath.Join(config, "shortcuts.vdf"))
	if err != nil {
		t.Fatalf("shortcuts.vdf missing: %v", err)
	}
	parsed, err := steam.ReadVDF(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse shortcuts.vdf: %v", err)
	}
	list, _ := parsed.Child("shortcuts")
	shortcut, ok := list.Child("0")
	if !ok {
		t.Fatal("shortcut missing")
	}
	if shortcut.String("AppName") != "Лолка" {
		t.Errorf("AppName = %q", shortcut.String("AppName"))
	}
	if shortcut.String("icon") != filepath.Join(prefix, "usr/share/icons/hicolor/48x48/apps/lolka.png") {
		t.Errorf("icon = %q", shortcut.String("icon"))
	}
	if result.Accounts[0] != "123456" {
		t.Errorf("accounts = %v", result.Accounts)
	}
}

func TestAdaptWithoutSteamProfile(t *testing.T) {
	_, err := steam.Adapt(steam.Request{
		Home:        t.TempDir(),
		PackageName: "lolka",
		PayloadDir:  samplePayload(t),
		Exec:        "/usr/bin/lolka",
	})
	if err == nil || !strings.Contains(err.Error(), "Steam") {
		t.Fatalf("expected a missing Steam error, got %v", err)
	}
}
