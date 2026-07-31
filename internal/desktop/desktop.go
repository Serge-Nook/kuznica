// Package desktop discovers, generates and validates freedesktop.org
// Desktop Entry files for the converted package.
package desktop

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Entry is a Desktop Entry as edited by the user.
type Entry struct {
	Name          string
	GenericName   string
	Comment       string
	Exec          string
	Icon          string
	Categories    string
	Terminal      bool
	StartupNotify bool
	Type          string
}

// ExistingAction describes what to do when the package already ships a
// .desktop file.
type ExistingAction string

const (
	ActionUseExisting ExistingAction = "use-existing"
	ActionReplace     ExistingAction = "replace"
	ActionCreateNew   ExistingAction = "create-new"
	ActionEdit        ExistingAction = "edit"
)

// Render serialises the entry in Desktop Entry format.
func (e Entry) Render() string {
	entryType := e.Type
	if entryType == "" {
		entryType = "Application"
	}
	var b strings.Builder
	b.WriteString("[Desktop Entry]\n")
	fmt.Fprintf(&b, "Type=%s\n", entryType)
	fmt.Fprintf(&b, "Name=%s\n", oneLine(e.Name))
	if e.GenericName != "" {
		fmt.Fprintf(&b, "GenericName=%s\n", oneLine(e.GenericName))
	}
	if e.Comment != "" {
		fmt.Fprintf(&b, "Comment=%s\n", oneLine(e.Comment))
	}
	fmt.Fprintf(&b, "Exec=%s\n", oneLine(e.Exec))
	if e.Icon != "" {
		fmt.Fprintf(&b, "Icon=%s\n", oneLine(e.Icon))
	}
	fmt.Fprintf(&b, "Terminal=%s\n", boolString(e.Terminal))
	fmt.Fprintf(&b, "StartupNotify=%s\n", boolString(e.StartupNotify))
	fmt.Fprintf(&b, "Categories=%s\n", normalizeCategories(e.Categories))
	return b.String()
}

// Parse reads a Desktop Entry file content into an Entry.
func Parse(content string) Entry {
	entry := Entry{Type: "Application"}
	scanner := bufio.NewScanner(strings.NewReader(content))
	inSection := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") {
			inSection = line == "[Desktop Entry]"
			continue
		}
		if !inSection || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		switch strings.TrimSpace(key) {
		case "Name":
			entry.Name = value
		case "GenericName":
			entry.GenericName = value
		case "Comment":
			entry.Comment = value
		case "Exec":
			entry.Exec = value
		case "Icon":
			entry.Icon = value
		case "Categories":
			entry.Categories = value
		case "Terminal":
			entry.Terminal = value == "true"
		case "StartupNotify":
			entry.StartupNotify = value == "true"
		case "Type":
			entry.Type = value
		}
	}
	return entry
}

// FindExisting returns the .desktop files shipped inside the extracted
// Debian payload.
func FindExisting(payloadDir string) []string {
	var found []string
	roots := []string{
		filepath.Join(payloadDir, "usr", "share", "applications"),
		filepath.Join(payloadDir, "usr", "local", "share", "applications"),
		filepath.Join(payloadDir, "opt"),
	}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(path, ".desktop") {
				found = append(found, path)
			}
			return nil
		})
	}
	sort.Strings(found)
	return found
}

// FindIcon looks for the most suitable icon inside the payload and returns
// the icon name usable in the Icon= field plus the file it was found at.
func FindIcon(payloadDir, packageName string) (name string, path string) {
	var candidates []string
	roots := []string{
		filepath.Join(payloadDir, "usr", "share", "icons"),
		filepath.Join(payloadDir, "usr", "share", "pixmaps"),
		filepath.Join(payloadDir, "opt"),
	}
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			switch strings.ToLower(filepath.Ext(p)) {
			case ".png", ".svg", ".xpm":
				candidates = append(candidates, p)
			}
			return nil
		})
	}
	if len(candidates) == 0 {
		return "", ""
	}
	sort.Slice(candidates, func(i, j int) bool {
		return iconScore(candidates[i], packageName) > iconScore(candidates[j], packageName)
	})
	best := candidates[0]
	base := filepath.Base(best)
	return strings.TrimSuffix(base, filepath.Ext(base)), best
}

func iconScore(path, packageName string) int {
	base := strings.ToLower(filepath.Base(path))
	score := 0
	if strings.HasPrefix(base, strings.ToLower(packageName)) {
		score += 100
	}
	if strings.Contains(path, "scalable") || strings.HasSuffix(base, ".svg") {
		score += 40
	}
	for _, size := range []string{"512x512", "256x256", "128x128", "64x64", "48x48"} {
		if strings.Contains(path, size) {
			score += 30
			break
		}
	}
	if strings.Contains(path, "pixmaps") {
		score += 10
	}
	return score - len(path)/100
}

// CategoryForSection maps a Debian section onto freedesktop categories.
func CategoryForSection(section string) string {
	section = strings.ToLower(section)
	if idx := strings.LastIndex(section, "/"); idx >= 0 {
		section = section[idx+1:]
	}
	switch section {
	case "devel", "python", "java", "haskell", "ruby":
		return "Development;"
	case "editors", "text", "tex":
		return "Utility;TextEditor;"
	case "graphics":
		return "Graphics;"
	case "sound", "video":
		return "AudioVideo;"
	case "net", "web", "comm", "mail", "news":
		return "Network;"
	case "games":
		return "Game;"
	case "science", "math", "electronics":
		return "Education;Science;"
	case "admin", "utils", "shells", "otherosfs":
		return "Utility;System;"
	case "doc":
		return "Documentation;"
	case "office", "database":
		return "Office;"
	default:
		return "Utility;"
	}
}

// Suggest builds a Desktop Entry from the analysed package payload.
func Suggest(payloadDir, packageName, description, section string, executables []string, detectIcons bool) Entry {
	exec := "/usr/bin/" + packageName
	if len(executables) > 0 {
		exec = "/" + executables[0]
		for _, candidate := range executables {
			if strings.EqualFold(filepath.Base(candidate), packageName) {
				exec = "/" + candidate
				break
			}
		}
	}
	entry := Entry{
		Type:          "Application",
		Name:          strings.ToUpper(packageName[:1]) + packageName[1:],
		Comment:       oneLine(description),
		Exec:          exec,
		Terminal:      false,
		StartupNotify: true,
		Categories:    CategoryForSection(section),
	}
	if detectIcons {
		if name, _ := FindIcon(payloadDir, packageName); name != "" {
			entry.Icon = name
		}
	}
	if entry.Icon == "" {
		entry.Icon = packageName
	}
	return entry
}

// Write saves the entry to path, creating parent directories.
func Write(path string, entry Entry) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(entry.Render()), 0o644)
}

// Validate runs desktop-file-validate on path. It returns the tool output
// and an error when the entry is rejected. A missing validator is reported
// through ErrValidatorMissing.
func Validate(ctx context.Context, path string) (string, error) {
	binary, err := exec.LookPath("desktop-file-validate")
	if err != nil {
		return "", ErrValidatorMissing
	}
	output, err := exec.CommandContext(ctx, binary, path).CombinedOutput()
	return string(output), err
}

// ErrValidatorMissing signals that desktop-file-validate is not installed.
var ErrValidatorMissing = fmt.Errorf("desktop-file-validate is not installed")

func boolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func oneLine(value string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(value, "\n", " ")), " ")
}

func normalizeCategories(categories string) string {
	categories = strings.TrimSpace(categories)
	if categories == "" {
		return "Utility;"
	}
	if !strings.HasSuffix(categories, ";") {
		categories += ";"
	}
	return categories
}
