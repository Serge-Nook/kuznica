// Package installer runs the external Arch Linux tools: makepkg for the
// build stage and pacman for the installation stage.
package installer

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Errors describing a missing or unusable Arch Linux tool chain.
var (
	ErrMakepkgMissing  = errors.New("makepkg is not installed (package base-devel)")
	ErrFakerootMissing = errors.New("fakeroot is not installed (package base-devel)")
	ErrPacmanMissing   = errors.New("pacman is not installed")
	ErrNoElevation     = errors.New("neither pkexec nor sudo is available for privilege elevation")
	ErrNoPackageBuilt  = errors.New("makepkg did not produce a .pkg.tar.zst file")
	ErrRunningAsRoot   = errors.New("makepkg refuses to run as root")
	ErrNotEnoughSpace  = errors.New("not enough free disk space")
	ErrBuildDirMissing = errors.New("build directory does not exist")
	ErrReadOnlyRoot    = errors.New("the pacman database is on a read-only filesystem")
)

// OutputFunc receives the command output line by line.
type OutputFunc func(line string)

// HasMakepkg reports whether makepkg is available.
func HasMakepkg() bool { return lookPath("makepkg") }

// HasPacman reports whether pacman is available.
func HasPacman() bool { return lookPath("pacman") }

// HasFakeroot reports whether fakeroot is available; makepkg aborts without it.
func HasFakeroot() bool { return lookPath("fakeroot") }

// BuildTools lists the binaries makepkg needs, all shipped by base-devel.
var BuildTools = []string{"makepkg", "fakeroot", "bsdtar"}

// MissingBuildTools returns the build tools that are not installed.
func MissingBuildTools() []string {
	var missing []string
	for _, tool := range BuildTools {
		if !lookPath(tool) {
			missing = append(missing, tool)
		}
	}
	return missing
}

func lookPath(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// FreeSpace returns the free space of the filesystem holding path.
func FreeSpace(path string) (int64, error) {
	var stat syscall.Statfs_t
	for path != "" {
		if err := syscall.Statfs(path, &stat); err == nil {
			return int64(stat.Bavail) * int64(stat.Bsize), nil
		}
		parent := filepath.Dir(path)
		if parent == path {
			break
		}
		path = parent
	}
	return 0, fmt.Errorf("cannot determine free space")
}

// EnsureSpace verifies that at least required bytes are available below dir.
func EnsureSpace(dir string, required int64) error {
	free, err := FreeSpace(dir)
	if err != nil {
		return nil // unable to measure: do not block the conversion
	}
	if free < required {
		return fmt.Errorf("%w: %d bytes required, %d available", ErrNotEnoughSpace, required, free)
	}
	return nil
}

// Makepkg builds the package described by the PKGBUILD in dir and returns
// the path of the produced .pkg.tar.zst archive.
func Makepkg(ctx context.Context, dir string, out OutputFunc) (string, error) {
	if _, err := os.Stat(filepath.Join(dir, "PKGBUILD")); err != nil {
		return "", ErrBuildDirMissing
	}
	if !HasMakepkg() {
		return "", ErrMakepkgMissing
	}
	if !HasFakeroot() {
		return "", ErrFakerootMissing
	}
	if os.Geteuid() == 0 {
		return "", ErrRunningAsRoot
	}

	cmd := exec.CommandContext(ctx, "makepkg", "--force", "--noconfirm", "--nodeps", "--needed")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "PKGDEST="+dir, "LC_ALL=C")

	var reported []string
	collect := func(line string) {
		if reason := MakepkgErrorLine(line); reason != "" {
			reported = append(reported, reason)
		}
		if out != nil {
			out(line)
		}
	}
	if err := run(cmd, collect); err != nil {
		return "", wrapMakepkgError(err, reported)
	}
	return FindBuiltPackage(dir)
}

// MakepkgErrorLine extracts the message of a makepkg "==> ERROR:" line.
func MakepkgErrorLine(line string) string {
	const marker = "ERROR:"
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "==>") {
		return ""
	}
	index := strings.Index(trimmed, marker)
	if index < 0 {
		return ""
	}
	return strings.TrimSpace(trimmed[index+len(marker):])
}

// wrapMakepkgError turns a bare "exit status N" into the reason makepkg printed.
func wrapMakepkgError(err error, reported []string) error {
	if len(reported) == 0 {
		return err
	}
	reason := reported[len(reported)-1]
	if strings.Contains(strings.ToLower(reason), "fakeroot") {
		return ErrFakerootMissing
	}
	return fmt.Errorf("makepkg: %s (%w)", reason, err)
}

// FindBuiltPackage returns the newest built package inside dir.
func FindBuiltPackage(dir string) (string, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.pkg.tar.zst"))
	if err != nil {
		return "", err
	}
	if len(matches) == 0 {
		if fallback, _ := filepath.Glob(filepath.Join(dir, "*.pkg.tar.*")); len(fallback) > 0 {
			matches = fallback
		} else {
			return "", ErrNoPackageBuilt
		}
	}
	sort.Slice(matches, func(i, j int) bool {
		return modTime(matches[i]).After(modTime(matches[j]))
	})
	return matches[0], nil
}

func modTime(path string) time.Time {
	info, err := os.Stat(path)
	if err != nil {
		return time.Time{}
	}
	return info.ModTime()
}

// Install installs a built package through pacman, elevating privileges
// with pkexec or sudo. КУЗНИЦА itself never runs as root.
func Install(ctx context.Context, packagePath string, out OutputFunc) error {
	if !HasPacman() {
		return ErrPacmanMissing
	}
	name, args, err := elevate([]string{"pacman", "-U", "--noconfirm", packagePath})
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return runPacman(cmd, out)
}

// InstallDependencies installs the given Arch dependencies with pacman -S.
func InstallDependencies(ctx context.Context, deps []string, out OutputFunc) error {
	if len(deps) == 0 {
		return nil
	}
	if !HasPacman() {
		return ErrPacmanMissing
	}
	name, args, err := elevate(append([]string{"pacman", "-S", "--needed", "--noconfirm"}, stripConstraints(deps)...))
	if err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	return runPacman(cmd, out)
}

// runPacman runs a pacman command and reports a read-only root filesystem
// (SteamOS and other immutable images) instead of a bare exit status.
func runPacman(cmd *exec.Cmd, out OutputFunc) error {
	readOnly := false
	collect := func(line string) {
		if IsReadOnlyError(line) {
			readOnly = true
		}
		if out != nil {
			out(line)
		}
	}
	err := run(cmd, collect)
	if err != nil && readOnly {
		return ErrReadOnlyRoot
	}
	return err
}

// IsReadOnlyError reports whether a pacman line complains about a read-only
// filesystem.
func IsReadOnlyError(line string) bool {
	return strings.Contains(strings.ToLower(line), "read-only file system")
}

func stripConstraints(deps []string) []string {
	out := make([]string, 0, len(deps))
	for _, dep := range deps {
		parts := strings.FieldsFunc(dep, func(r rune) bool {
			return r == '>' || r == '<' || r == '='
		})
		if len(parts) == 0 {
			continue
		}
		out = append(out, parts[0])
	}
	return out
}

func elevate(command []string) (string, []string, error) {
	if os.Geteuid() == 0 {
		return command[0], command[1:], nil
	}
	for _, helper := range []string{"pkexec", "sudo"} {
		if lookPath(helper) {
			return helper, command, nil
		}
	}
	return "", nil, ErrNoElevation
}

// OpenFolder opens dir in the user's file manager.
func OpenFolder(ctx context.Context, dir string) error {
	binary, err := exec.LookPath("xdg-open")
	if err != nil {
		return fmt.Errorf("xdg-open is not installed")
	}
	return exec.CommandContext(ctx, binary, dir).Start()
}

func run(cmd *exec.Cmd, out OutputFunc) error {
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	for _, pipe := range []io.Reader{stdout, stderr} {
		go func(r io.Reader) {
			defer wg.Done()
			stream(r, out)
		}(pipe)
	}
	wg.Wait()
	return cmd.Wait()
}

func stream(r io.Reader, out OutputFunc) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		if out != nil {
			out(scanner.Text())
		}
	}
}
