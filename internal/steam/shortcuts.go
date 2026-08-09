package steam

import (
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// ErrNoSteam is returned when no Steam installation was found.
var ErrNoSteam = errors.New("no Steam installation with a user profile was found")

// Shortcut describes a non-Steam game entry.
type Shortcut struct {
	Name          string
	Exe           string // absolute path of the launcher
	StartDir      string
	Icon          string
	LaunchOptions string
	Tags          []string
}

// User is a Steam profile whose shortcuts can be modified.
type User struct {
	AccountID string // Steam3 account id, the userdata directory name
	ConfigDir string // <steam>/userdata/<id>/config
}

// ShortcutsPath returns the shortcuts.vdf path of the user.
func (u User) ShortcutsPath() string { return filepath.Join(u.ConfigDir, "shortcuts.vdf") }

// steamRoots lists the possible Steam installation directories, including the
// SteamOS and Flatpak layouts.
func steamRoots(home string) []string {
	return []string{
		filepath.Join(home, ".steam", "steam"),
		filepath.Join(home, ".steam", "root"),
		filepath.Join(home, ".local", "share", "Steam"),
		filepath.Join(home, ".var", "app", "com.valvesoftware.Steam", "data", "Steam"),
	}
}

// Users returns every Steam profile found below home.
func Users(home string) ([]User, error) {
	seen := map[string]bool{}
	var users []User
	for _, root := range steamRoots(home) {
		resolved, err := filepath.EvalSymlinks(root)
		if err != nil {
			continue
		}
		userdata := filepath.Join(resolved, "userdata")
		entries, err := os.ReadDir(userdata)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() || !isAccountID(entry.Name()) {
				continue
			}
			config := filepath.Join(userdata, entry.Name(), "config")
			if info, err := os.Stat(config); err != nil || !info.IsDir() {
				continue
			}
			if seen[config] {
				continue
			}
			seen[config] = true
			users = append(users, User{AccountID: entry.Name(), ConfigDir: config})
		}
	}
	if len(users) == 0 {
		return nil, ErrNoSteam
	}
	return users, nil
}

// isAccountID rejects the "0" and "anonymous" pseudo profiles.
func isAccountID(name string) bool {
	id, err := strconv.ParseUint(name, 10, 64)
	return err == nil && id > 0
}

// AppID computes the identifier Steam uses for a non-Steam shortcut. The
// same CRC32 based formula is used by Steam itself for the artwork names.
func AppID(exe, name string) int32 {
	sum := crc32.ChecksumIEEE([]byte(quote(exe) + name))
	return int32(sum | 0x80000000) //nolint:gosec // the high bit marks a shortcut
}

// AddShortcut registers or updates the shortcut in the profile. The previous
// shortcuts.vdf is backed up once before the first modification.
func AddShortcut(user User, shortcut Shortcut) (int32, error) {
	path := user.ShortcutsPath()
	root, err := loadShortcuts(path)
	if err != nil {
		return 0, err
	}
	list, ok := root.Child("shortcuts")
	if !ok {
		list = NewMap()
		root.Set("shortcuts", list)
	}

	appID := AppID(shortcut.Exe, shortcut.Name)
	entry := renderShortcut(shortcut, appID)
	if key, found := findShortcut(list, shortcut, appID); found {
		list.Set(key, entry)
	} else {
		list.Set(strconv.Itoa(nextIndex(list)), entry)
	}

	if err := backup(path); err != nil {
		return 0, err
	}
	if err := writeAtomic(path, root); err != nil {
		return 0, err
	}
	return appID, nil
}

// RemoveShortcut deletes a shortcut previously added for name/exe and reports
// whether anything was removed.
func RemoveShortcut(user User, shortcut Shortcut) (bool, error) {
	path := user.ShortcutsPath()
	root, err := loadShortcuts(path)
	if err != nil {
		return false, err
	}
	list, ok := root.Child("shortcuts")
	if !ok {
		return false, nil
	}
	key, found := findShortcut(list, shortcut, AppID(shortcut.Exe, shortcut.Name))
	if !found {
		return false, nil
	}
	list.Delete(key)
	reindex(list)
	if err := backup(path); err != nil {
		return false, err
	}
	if err := writeAtomic(path, root); err != nil {
		return false, err
	}
	return true, nil
}

func loadShortcuts(path string) (*Map, error) {
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		root := NewMap()
		root.Set("shortcuts", NewMap())
		return root, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	root, err := ReadVDF(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return root, nil
}

// findShortcut locates an existing entry by app id, or by name and launcher.
func findShortcut(list *Map, shortcut Shortcut, appID int32) (string, bool) {
	for _, key := range list.Keys() {
		entry, ok := list.Child(key)
		if !ok {
			continue
		}
		if id, ok := entry.Get("appid"); ok {
			if current, ok := id.(int32); ok && current == appID {
				return key, true
			}
		}
		if entry.String("AppName") == shortcut.Name &&
			unquote(entry.String("Exe")) == shortcut.Exe {
			return key, true
		}
	}
	return "", false
}

func nextIndex(list *Map) int {
	next := 0
	for _, key := range list.Keys() {
		if index, err := strconv.Atoi(key); err == nil && index >= next {
			next = index + 1
		}
	}
	return next
}

// reindex renumbers the entries so the indexes stay contiguous.
func reindex(list *Map) {
	entries := make([]*Map, 0, list.Len())
	for _, key := range list.Keys() {
		if entry, ok := list.Child(key); ok {
			entries = append(entries, entry)
		}
		list.Delete(key)
	}
	for index, entry := range entries {
		list.Set(strconv.Itoa(index), entry)
	}
}

// renderShortcut builds the entry with the fields Steam writes itself.
func renderShortcut(shortcut Shortcut, appID int32) *Map {
	entry := NewMap()
	entry.Set("appid", appID)
	entry.Set("AppName", shortcut.Name)
	entry.Set("Exe", quote(shortcut.Exe))
	entry.Set("StartDir", quote(shortcut.StartDir))
	entry.Set("icon", shortcut.Icon)
	entry.Set("ShortcutPath", "")
	entry.Set("LaunchOptions", shortcut.LaunchOptions)
	entry.Set("IsHidden", int32(0))
	entry.Set("AllowDesktopConfig", int32(1))
	entry.Set("AllowOverlay", int32(1))
	entry.Set("OpenVR", int32(0))
	entry.Set("Devkit", int32(0))
	entry.Set("DevkitGameID", "")
	entry.Set("DevkitOverrideAppID", int32(0))
	entry.Set("LastPlayTime", int32(0))
	entry.Set("FlatpakAppID", "")

	tags := NewMap()
	for index, tag := range shortcut.Tags {
		tags.Set(strconv.Itoa(index), tag)
	}
	entry.Set("tags", tags)
	return entry
}

// backup copies the original shortcuts.vdf once, before КУЗНИЦА touches it.
func backup(path string) error {
	target := path + ".kuznica.bak"
	if _, err := os.Stat(target); err == nil {
		return nil // the pre-KUZNICA state is already preserved
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.WriteFile(target, data, 0o600)
}

func writeAtomic(path string, root *Map) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".shortcuts-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())

	if err := WriteVDF(temp, root); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), 0o600); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}

// quote wraps a path the way Steam stores Exe and StartDir.
func quote(value string) string {
	if strings.HasPrefix(value, `"`) {
		return value
	}
	return `"` + value + `"`
}

func unquote(value string) string { return strings.Trim(value, `"`) }
