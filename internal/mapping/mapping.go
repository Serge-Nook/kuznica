// Package mapping translates Debian package names and dependency
// expressions into their Arch Linux counterparts.
package mapping

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/Serge-Nook/kuznica/assets"
)

// Database holds the Debian ↔ Arch package name correspondence.
type Database struct {
	mu       sync.RWMutex
	packages map[string]string
	ignored  map[string]bool
}

type fileFormat struct {
	Packages map[string]string `json:"packages"`
	Ignored  []string          `json:"ignored"`
}

// Result describes the translation of a single Debian dependency.
type Result struct {
	Debian     string // original expression, e.g. "libgtk-3-0 (>= 3.24)"
	DebianName string // bare Debian package name
	Arch       string // resulting Arch dependency, empty when dropped
	Constraint string // version constraint carried over, e.g. ">=3.24"
	Optional   bool   // dependency came from an alternatives list
	Mapped     bool   // an explicit database entry was used
	Ignored    bool   // dependency is provided by the Arch base system
}

// NewDefault loads the mapping database embedded into the binary.
func NewDefault() (*Database, error) {
	db := &Database{packages: map[string]string{}, ignored: map[string]bool{}}
	if err := db.merge(assets.DefaultMappings); err != nil {
		return nil, err
	}
	return db, nil
}

// Load returns the embedded database merged with the user database at path.
// A missing user file is not an error.
func Load(path string) (*Database, error) {
	db, err := NewDefault()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return db, nil
		}
		return db, err
	}
	if err := db.merge(data); err != nil {
		return db, err
	}
	return db, nil
}

// UserPath is the location of the user editable mapping database.
func UserPath() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "kuznica", "mappings.json")
}

func (d *Database) merge(data []byte) error {
	var parsed fileFormat
	if err := json.Unmarshal(data, &parsed); err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	for deb, arch := range parsed.Packages {
		d.packages[strings.ToLower(deb)] = arch
	}
	for _, name := range parsed.Ignored {
		d.ignored[strings.ToLower(name)] = true
	}
	return nil
}

// Set adds or replaces a single mapping.
func (d *Database) Set(debian, arch string) {
	d.mu.Lock()
	d.packages[strings.ToLower(debian)] = arch
	d.mu.Unlock()
}

// Len reports the number of known mappings.
func (d *Database) Len() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.packages)
}

// Save writes the database to path in the user editable JSON format.
func (d *Database) Save(path string) error {
	d.mu.RLock()
	out := fileFormat{Packages: make(map[string]string, len(d.packages))}
	for k, v := range d.packages {
		out.Packages[k] = v
	}
	for name := range d.ignored {
		out.Ignored = append(out.Ignored, name)
	}
	d.mu.RUnlock()
	sort.Strings(out.Ignored)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// Translate converts one Debian dependency expression. Alternatives
// ("foo | bar") are resolved to the first known alternative.
func (d *Database) Translate(expr string) Result {
	expr = strings.TrimSpace(expr)
	res := Result{Debian: expr}
	if expr == "" {
		return res
	}

	alternatives := strings.Split(expr, "|")
	res.Optional = len(alternatives) > 1

	var fallback Result
	for i, alt := range alternatives {
		candidate := d.translateSingle(strings.TrimSpace(alt))
		if candidate.Mapped || candidate.Ignored {
			candidate.Debian = expr
			candidate.Optional = res.Optional
			return candidate
		}
		if i == 0 {
			fallback = candidate
		}
	}
	fallback.Debian = expr
	fallback.Optional = res.Optional
	return fallback
}

func (d *Database) translateSingle(expr string) Result {
	name, constraint := splitConstraint(expr)
	res := Result{Debian: expr, DebianName: name, Constraint: constraint}
	if name == "" {
		return res
	}

	key := strings.ToLower(name)
	d.mu.RLock()
	arch, mapped := d.packages[key]
	ignored := d.ignored[key]
	d.mu.RUnlock()

	switch {
	case ignored:
		res.Ignored = true
		return res
	case mapped:
		res.Mapped = true
		if arch == "" { // explicit drop
			res.Ignored = true
			return res
		}
		res.Arch = arch
	default:
		res.Arch = guessArchName(name)
	}

	if res.Constraint != "" {
		res.Arch += res.Constraint
	}
	return res
}

// TranslateList converts a comma separated Debian dependency field.
func (d *Database) TranslateList(field string) []Result {
	var out []Result
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, d.Translate(part))
	}
	return out
}

// ArchDepends returns the deduplicated Arch dependency list for a Debian
// dependency field.
func (d *Database) ArchDepends(field string) []string {
	seen := map[string]bool{}
	var out []string
	for _, res := range d.TranslateList(field) {
		if res.Ignored || res.Arch == "" || seen[res.Arch] {
			continue
		}
		seen[res.Arch] = true
		out = append(out, res.Arch)
	}
	return out
}

// splitConstraint splits "name (>= 1.2)" into the name and ">=1.2".
func splitConstraint(expr string) (string, string) {
	expr = strings.TrimSpace(expr)
	open := strings.Index(expr, "(")
	if open < 0 {
		return strings.TrimSpace(stripArchQualifier(expr)), ""
	}
	name := strings.TrimSpace(stripArchQualifier(expr[:open]))
	close := strings.Index(expr[open:], ")")
	if close < 0 {
		return name, ""
	}
	inner := strings.TrimSpace(expr[open+1 : open+close])
	inner = strings.TrimPrefix(inner, "=")
	for _, op := range []string{">>", "<<", ">=", "<=", "="} {
		if strings.HasPrefix(inner, op) {
			version := strings.TrimSpace(strings.TrimPrefix(inner, op))
			return name, normalizeOperator(op) + sanitizeVersion(version)
		}
	}
	return name, ""
}

func normalizeOperator(op string) string {
	switch op {
	case ">>":
		return ">"
	case "<<":
		return "<"
	default:
		return op
	}
}

// stripArchQualifier removes multi-arch qualifiers such as ":any".
func stripArchQualifier(name string) string {
	if idx := strings.Index(name, ":"); idx >= 0 {
		return name[:idx]
	}
	return name
}

// sanitizeVersion drops the Debian revision and epoch that pacman cannot use.
func sanitizeVersion(version string) string {
	if idx := strings.Index(version, ":"); idx >= 0 {
		version = version[idx+1:]
	}
	if idx := strings.Index(version, "-"); idx >= 0 {
		version = version[:idx]
	}
	if idx := strings.Index(version, "~"); idx >= 0 {
		version = version[:idx]
	}
	return strings.TrimSpace(version)
}

// guessArchName applies the common Debian naming conventions when the
// database has no explicit entry: libfoo1 → libfoo, foo-dev → foo.
func guessArchName(name string) string {
	name = strings.ToLower(name)
	name = strings.TrimSuffix(name, ":any")
	trimmed := strings.TrimRight(name, "0123456789.")
	if trimmed != "" && strings.HasPrefix(trimmed, "lib") {
		name = trimmed
	}
	name = strings.TrimSuffix(name, "-dev")
	return strings.TrimSuffix(name, "-")
}
