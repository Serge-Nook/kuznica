package tests

import (
	"archive/tar"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/klauspost/compress/zstd"

	"github.com/Serge-Nook/kuznica/internal/config"
	"github.com/Serge-Nook/kuznica/internal/converter"
	"github.com/Serge-Nook/kuznica/internal/logger"
	"github.com/Serge-Nook/kuznica/internal/mapping"
)

func newConverter(t *testing.T) (*converter.Converter, *logger.Logger) {
	t.Helper()
	cfg := config.Default()
	cfg.OutputDir = t.TempDir()
	cfg.UseMakepkg = false
	cfg.CreateDesktop = true
	cfg.ValidateDesktop = false

	db, err := mapping.NewDefault()
	if err != nil {
		t.Fatalf("mappings: %v", err)
	}
	log := logger.New()
	return converter.New(cfg, log, db), log
}

func TestConvertGeneratesBuildFiles(t *testing.T) {
	conv, log := newConverter(t)
	state, err := conv.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer state.Package.Cleanup()

	if err := conv.Convert(state); err != nil {
		t.Fatalf("convert: %v", err)
	}

	pkgbuildData, err := os.ReadFile(state.PKGBUILDPath)
	if err != nil {
		t.Fatalf("read PKGBUILD: %v", err)
	}
	text := string(pkgbuildData)
	for _, want := range []string{"pkgname=hello-world", "pkgver=2.10", "glibc>=2.34", "gtk3", "install=hello-world.install"} {
		if !strings.Contains(text, want) {
			t.Errorf("PKGBUILD does not contain %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "'apt'") {
		t.Error("Debian-only dependency apt must be dropped")
	}

	if _, err := os.Stat(state.SRCINFOPath); err != nil {
		t.Errorf(".SRCINFO missing: %v", err)
	}
	if _, err := os.Stat(state.InstallPath); err != nil {
		t.Errorf(".install missing: %v", err)
	}
	payload := filepath.Join(state.BuildDir, "payload", "usr", "bin", "hello-world")
	if _, err := os.Stat(payload); err != nil {
		t.Errorf("payload not staged: %v", err)
	}
	if state.DesktopEntry.Exec == "" {
		t.Error("desktop entry was not prepared")
	}
	if !strings.Contains(log.Text(), "Conversion of hello-world finished") {
		t.Errorf("journal does not record the conversion:\n%s", log.Text())
	}
}

func TestBuildWithoutMakepkgUsesBuiltInPackager(t *testing.T) {
	conv, _ := newConverter(t)
	state, err := conv.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer state.Package.Cleanup()
	if err := conv.Convert(state); err != nil {
		t.Fatalf("convert: %v", err)
	}
	if err := conv.Build(t.Context(), state); err != nil {
		t.Fatalf("build: %v", err)
	}
	if !strings.HasSuffix(state.ArtifactPath, ".pkg.tar.zst") {
		t.Fatalf("unexpected artifact %q", state.ArtifactPath)
	}

	members := packageMembers(t, state.ArtifactPath)
	for _, want := range []string{".PKGINFO", ".MTREE", "usr/bin/hello-world"} {
		if _, ok := members[want]; !ok {
			t.Errorf("%s missing from the package", want)
		}
	}
	info := members[".PKGINFO"]
	for _, want := range []string{"pkgname = hello-world", "arch = x86_64", "size = "} {
		if !strings.Contains(info, want) {
			t.Errorf(".PKGINFO does not contain %q:\n%s", want, info)
		}
	}
}

// packageMembers reads a .pkg.tar.zst archive into a name → contents map.
func packageMembers(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open package: %v", err)
	}
	defer file.Close()

	decoder, err := zstd.NewReader(file)
	if err != nil {
		t.Fatalf("zstd: %v", err)
	}
	defer decoder.Close()

	members := map[string]string{}
	reader := tar.NewReader(decoder)
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		body, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("read %s: %v", header.Name, err)
		}
		members[strings.TrimSuffix(header.Name, "/")] = string(body)
		if header.Uid != 0 || header.Gid != 0 {
			t.Errorf("%s is owned by %d:%d, want root", header.Name, header.Uid, header.Gid)
		}
	}
	return members
}

func TestInstallRequiresBuild(t *testing.T) {
	conv, _ := newConverter(t)
	state, err := conv.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer state.Package.Cleanup()
	if err := conv.Install(t.Context(), state); err == nil {
		t.Fatal("installing an unbuilt package must fail")
	}
}

func TestSaveDesktopEntry(t *testing.T) {
	conv, _ := newConverter(t)
	state, err := conv.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer state.Package.Cleanup()
	if err := conv.Convert(state); err != nil {
		t.Fatalf("convert: %v", err)
	}

	entry := state.DesktopEntry
	entry.Name = "Hello Kuznica"
	if err := conv.SaveDesktopEntry(t.Context(), state, entry); err != nil {
		t.Fatalf("save desktop entry: %v", err)
	}
	data, err := os.ReadFile(state.DesktopPath)
	if err != nil {
		t.Fatalf("read desktop entry: %v", err)
	}
	if !strings.Contains(string(data), "Name=Hello Kuznica") {
		t.Errorf("unexpected desktop entry:\n%s", data)
	}
}

func TestCleanupRemovesTempFiles(t *testing.T) {
	conv, _ := newConverter(t)
	state, err := conv.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	tempDir := state.Package.TempDir()
	conv.Cleanup(state)
	if _, err := os.Stat(tempDir); !os.IsNotExist(err) {
		t.Errorf("temporary directory %s still exists", tempDir)
	}
}
