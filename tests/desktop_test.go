package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Serge-Nook/kuznica/internal/desktop"
)

func TestRenderAndParseRoundTrip(t *testing.T) {
	entry := desktop.Entry{
		Name:          "Hello",
		Comment:       "Example greeting program",
		Exec:          "/usr/bin/hello-world",
		Icon:          "hello-world",
		Categories:    "Utility",
		Terminal:      false,
		StartupNotify: true,
	}
	rendered := entry.Render()
	if !strings.HasPrefix(rendered, "[Desktop Entry]\n") {
		t.Fatalf("missing group header:\n%s", rendered)
	}
	if !strings.Contains(rendered, "Categories=Utility;") {
		t.Errorf("categories must be terminated by a semicolon:\n%s", rendered)
	}

	parsed := desktop.Parse(rendered)
	if parsed.Name != entry.Name || parsed.Exec != entry.Exec || parsed.Icon != entry.Icon {
		t.Errorf("round trip mismatch: %+v", parsed)
	}
	if !parsed.StartupNotify || parsed.Terminal {
		t.Errorf("boolean fields lost: %+v", parsed)
	}
}

func TestSuggestUsesPayload(t *testing.T) {
	payload := t.TempDir()
	iconPath := filepath.Join(payload, "usr", "share", "icons", "hicolor", "128x128", "apps")
	if err := os.MkdirAll(iconPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(iconPath, "hello-world.png"), []byte("PNG"), 0o644); err != nil {
		t.Fatalf("write icon: %v", err)
	}

	entry := desktop.Suggest(payload, "hello-world", "Example greeting program", "utils",
		[]string{"usr/bin/hello-world"}, true)
	if entry.Exec != "/usr/bin/hello-world" {
		t.Errorf("Exec = %q", entry.Exec)
	}
	if entry.Icon != "hello-world" {
		t.Errorf("Icon = %q", entry.Icon)
	}
	if entry.Categories != "Utility;System;" {
		t.Errorf("Categories = %q", entry.Categories)
	}
}

func TestCategoryForSection(t *testing.T) {
	cases := map[string]string{
		"devel":          "Development;",
		"games":          "Game;",
		"graphics":       "Graphics;",
		"contrib/net":    "Network;",
		"totally-random": "Utility;",
	}
	for section, want := range cases {
		if got := desktop.CategoryForSection(section); got != want {
			t.Errorf("CategoryForSection(%q) = %q, want %q", section, got, want)
		}
	}
}

func TestFindExisting(t *testing.T) {
	payload := t.TempDir()
	dir := filepath.Join(payload, "usr", "share", "applications")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := filepath.Join(dir, "hello.desktop")
	if err := os.WriteFile(path, []byte("[Desktop Entry]\nName=Hello\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	found := desktop.FindExisting(payload)
	if len(found) != 1 || found[0] != path {
		t.Fatalf("FindExisting = %v", found)
	}
}

func TestWriteCreatesParents(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "hello.desktop")
	if err := desktop.Write(path, desktop.Entry{Name: "Hello", Exec: "/usr/bin/hello"}); err != nil {
		t.Fatalf("write: %v", err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "Name=Hello") {
		t.Errorf("unexpected content: %s", data)
	}
}
