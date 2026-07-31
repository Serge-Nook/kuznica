// Package deb reads Debian binary packages: it validates the ar container,
// extracts control.tar and data.tar into a private temporary directory and
// exposes the package metadata.
package deb

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/blakesmith/ar"
)

// Errors reported for malformed or unsupported packages.
var (
	ErrNotDebPackage  = errors.New("file is not a Debian package")
	ErrNoControlTar   = errors.New("control.tar is missing from the package")
	ErrNoDataTar      = errors.New("data.tar is missing from the package")
	ErrNoDebianBinary = errors.New("debian-binary is missing from the package")
	ErrPathTraversal  = errors.New("archive member escapes the extraction directory")
)

// MaxPackageSize is the largest package КУЗНИЦА accepts (5 GiB).
const MaxPackageSize = 5 << 30

// Package is an opened Debian package.
type Package struct {
	SourcePath    string
	FileSize      int64
	FormatVersion string

	Name         string
	Version      string
	Architecture string
	Description  string
	Maintainer   string
	Homepage     string
	License      string
	Section      string
	Priority     string
	Essential    string
	// InstalledSize is the Installed-Size control field in kibibytes.
	InstalledSize int64

	Depends    []string
	PreDepends []string
	Recommends []string
	Suggests   []string
	Conflicts  []string
	Breaks     []string
	Replaces   []string
	Provides   []string

	Control   Control
	Checksums map[string]string
	// Scripts maps maintainer script names (preinst, postinst, prerm,
	// postrm) to their contents.
	Scripts map[string]string

	Files []FileEntry

	tempDir    string
	dataDir    string
	controlDir string
}

// DataDir is the directory holding the extracted data.tar payload.
func (p *Package) DataDir() string { return p.dataDir }

// ControlDir is the directory holding the extracted control.tar payload.
func (p *Package) ControlDir() string { return p.controlDir }

// TempDir is the private extraction directory of this package.
func (p *Package) TempDir() string { return p.tempDir }

// Cleanup removes the temporary extraction directory.
func (p *Package) Cleanup() error {
	if p.tempDir == "" {
		return nil
	}
	dir := p.tempDir
	p.tempDir = ""
	return os.RemoveAll(dir)
}

// Open reads path and extracts it into a fresh temporary directory.
func Open(path string) (*Package, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, ErrNotDebPackage
	}
	if info.Size() > MaxPackageSize {
		return nil, fmt.Errorf("package is larger than %d bytes", int64(MaxPackageSize))
	}

	tempDir, err := os.MkdirTemp("", "kuznica-")
	if err != nil {
		return nil, err
	}

	pkg := &Package{
		SourcePath: path,
		FileSize:   info.Size(),
		tempDir:    tempDir,
		dataDir:    filepath.Join(tempDir, "data"),
		controlDir: filepath.Join(tempDir, "control"),
		Checksums:  map[string]string{},
		Scripts:    map[string]string{},
	}

	if err := pkg.extract(path); err != nil {
		pkg.Cleanup()
		return nil, err
	}
	if err := pkg.loadMetadata(); err != nil {
		pkg.Cleanup()
		return nil, err
	}
	return pkg, nil
}

func (p *Package) extract(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	reader := ar.NewReader(f)
	var sawControl, sawData, sawBinary bool

	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			if !sawBinary {
				return ErrNotDebPackage
			}
			return fmt.Errorf("read ar container: %w", err)
		}

		name := strings.TrimRight(strings.TrimSpace(header.Name), "/")
		switch {
		case name == "debian-binary":
			data, err := io.ReadAll(reader)
			if err != nil {
				return err
			}
			p.FormatVersion = strings.TrimSpace(string(data))
			if !strings.HasPrefix(p.FormatVersion, "2.") {
				return fmt.Errorf("%w: unsupported format %q", ErrNotDebPackage, p.FormatVersion)
			}
			sawBinary = true
		case strings.HasPrefix(name, "control.tar"):
			if _, err := p.extractMember(name, reader, p.controlDir); err != nil {
				return err
			}
			sawControl = true
		case strings.HasPrefix(name, "data.tar"):
			files, err := p.extractMember(name, reader, p.dataDir)
			if err != nil {
				return err
			}
			p.Files = files
			sawData = true
		}
	}

	switch {
	case !sawBinary:
		return ErrNoDebianBinary
	case !sawControl:
		return ErrNoControlTar
	case !sawData:
		return ErrNoDataTar
	}
	return nil
}

func (p *Package) extractMember(name string, r io.Reader, dest string) ([]FileEntry, error) {
	stream, closer, err := decompress(name, r)
	if err != nil {
		return nil, err
	}
	defer closer.Close()
	return extractTar(stream, dest)
}

func (p *Package) loadMetadata() error {
	controlText, err := os.ReadFile(filepath.Join(p.controlDir, "control"))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNoControlTar, err)
	}
	ctrl := ParseControl(string(controlText))
	p.Control = ctrl

	p.Name = ctrl.Get("package")
	p.Version = ctrl.Get("version")
	p.Architecture = ctrl.Get("architecture")
	p.Description = ctrl.Get("description")
	p.Maintainer = ctrl.Get("maintainer")
	p.Homepage = ctrl.Get("homepage")
	p.Section = ctrl.Get("section")
	p.Priority = ctrl.Get("priority")
	p.Essential = ctrl.Get("essential")
	p.License = p.detectLicense()

	if size := ctrl.Get("installed-size"); size != "" {
		p.InstalledSize, _ = strconv.ParseInt(strings.TrimSpace(size), 10, 64)
	}

	p.Depends = SplitDependencies(ctrl.Get("depends"))
	p.PreDepends = SplitDependencies(ctrl.Get("pre-depends"))
	p.Recommends = SplitDependencies(ctrl.Get("recommends"))
	p.Suggests = SplitDependencies(ctrl.Get("suggests"))
	p.Conflicts = SplitDependencies(ctrl.Get("conflicts"))
	p.Breaks = SplitDependencies(ctrl.Get("breaks"))
	p.Replaces = SplitDependencies(ctrl.Get("replaces"))
	p.Provides = SplitDependencies(ctrl.Get("provides"))

	if p.Name == "" {
		return fmt.Errorf("%w: control file has no Package field", ErrNotDebPackage)
	}

	if sums, err := os.ReadFile(filepath.Join(p.controlDir, "md5sums")); err == nil {
		p.Checksums = ParseMD5Sums(string(sums))
	}
	for _, script := range []string{"preinst", "postinst", "prerm", "postrm"} {
		data, err := os.ReadFile(filepath.Join(p.controlDir, script))
		if err != nil {
			continue
		}
		p.Scripts[script] = string(data)
	}

	sort.Slice(p.Files, func(i, j int) bool { return p.Files[i].Path < p.Files[j].Path })
	return nil
}

// detectLicense derives the license name from the Debian copyright file.
func (p *Package) detectLicense() string {
	pattern := filepath.Join(p.dataDir, "usr", "share", "doc", "*", "copyright")
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return "custom"
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		return "custom"
	}
	for _, line := range strings.Split(string(data), "\n") {
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "License:"); ok {
			if license := strings.TrimSpace(value); license != "" {
				return license
			}
		}
	}
	return "custom"
}

// UpstreamVersion strips the Debian revision and epoch from the version.
func (p *Package) UpstreamVersion() string {
	version := p.Version
	if idx := strings.Index(version, ":"); idx >= 0 {
		version = version[idx+1:]
	}
	if idx := strings.LastIndex(version, "-"); idx >= 0 {
		version = version[:idx]
	}
	version = strings.ReplaceAll(version, "~", ".")
	if version == "" {
		return "0"
	}
	return version
}

// InstalledBytes returns the installed size in bytes.
func (p *Package) InstalledBytes() int64 { return p.InstalledSize * 1024 }

// Executables lists the regular executable files shipped by the package
// which are meant to end up on PATH.
func (p *Package) Executables() []string {
	var out []string
	for _, file := range p.Files {
		if file.IsDir {
			continue
		}
		dir := parentDir(file.Path)
		if dir != "usr/bin" && dir != "bin" && dir != "usr/sbin" && dir != "usr/games" && dir != "usr/local/bin" {
			continue
		}
		if file.Mode.Perm()&0o111 == 0 && !file.IsLink {
			continue
		}
		out = append(out, file.Path)
	}
	sort.Strings(out)
	return out
}

func parentDir(p string) string {
	idx := strings.LastIndex(p, "/")
	if idx < 0 {
		return ""
	}
	return p[:idx]
}

// VerifyChecksums recomputes the MD5 sums of the extracted payload and
// returns the paths whose checksum does not match the control archive.
func (p *Package) VerifyChecksums() ([]string, error) {
	if len(p.Checksums) == 0 {
		return nil, nil
	}
	var mismatched []string
	for file, expected := range p.Checksums {
		full, err := safeJoin(p.dataDir, file)
		if err != nil {
			return nil, err
		}
		actual, err := md5File(full)
		if err != nil {
			mismatched = append(mismatched, file)
			continue
		}
		if !strings.EqualFold(actual, expected) {
			mismatched = append(mismatched, file)
		}
	}
	sort.Strings(mismatched)
	return mismatched, nil
}

func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	hash := md5.New()
	if _, err := io.Copy(hash, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(hash.Sum(nil)), nil
}

// HumanSize renders a byte count using binary units.
func HumanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	units := []string{"KiB", "MiB", "GiB", "TiB"}
	if exp >= len(units) {
		exp = len(units) - 1
	}
	return fmt.Sprintf("%.1f %s", float64(bytes)/float64(div), units[exp])
}
