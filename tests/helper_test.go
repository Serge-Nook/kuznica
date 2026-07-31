package tests

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/blakesmith/ar"
	"github.com/klauspost/compress/zstd"
	"github.com/ulikunitz/xz"
)

// tarFile is a member of a generated fixture archive.
type tarFile struct {
	Name string
	Body string
	Mode int64
	Dir  bool
}

func buildTar(t *testing.T, files []tarFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, file := range files {
		mode := file.Mode
		if mode == 0 {
			mode = 0o644
		}
		header := &tar.Header{
			Name:     file.Name,
			Mode:     mode,
			Size:     int64(len(file.Body)),
			ModTime:  time.Unix(0, 0),
			Typeflag: tar.TypeReg,
		}
		if file.Dir {
			header.Typeflag = tar.TypeDir
			header.Size = 0
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatalf("write tar header: %v", err)
		}
		if !file.Dir {
			if _, err := tw.Write([]byte(file.Body)); err != nil {
				t.Fatalf("write tar body: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	return buf.Bytes()
}

func compressGzip(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(data); err != nil {
		t.Fatalf("gzip: %v", err)
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}

func compressXZ(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := xz.NewWriter(&buf)
	if err != nil {
		t.Fatalf("xz: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("xz write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("xz close: %v", err)
	}
	return buf.Bytes()
}

func compressZstd(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w, err := zstd.NewWriter(&buf)
	if err != nil {
		t.Fatalf("zstd: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("zstd write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("zstd close: %v", err)
	}
	return buf.Bytes()
}

// debFixture describes the archive members of a generated .deb file.
type debFixture struct {
	DebianBinary string
	ControlName  string
	ControlData  []byte
	DataName     string
	DataData     []byte
	SkipControl  bool
	SkipData     bool
}

func writeDeb(t *testing.T, fixture debFixture) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.deb")
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create deb: %v", err)
	}
	defer f.Close()

	writer := ar.NewWriter(f)
	if err := writer.WriteGlobalHeader(); err != nil {
		t.Fatalf("ar header: %v", err)
	}
	binary := fixture.DebianBinary
	if binary == "" {
		binary = "2.0\n"
	}
	addMember(t, writer, "debian-binary", []byte(binary))
	if !fixture.SkipControl {
		addMember(t, writer, fixture.ControlName, fixture.ControlData)
	}
	if !fixture.SkipData {
		addMember(t, writer, fixture.DataName, fixture.DataData)
	}
	return path
}

func addMember(t *testing.T, writer *ar.Writer, name string, body []byte) {
	t.Helper()
	header := &ar.Header{Name: name, Size: int64(len(body)), Mode: 0o644, ModTime: time.Unix(0, 0)}
	if err := writer.WriteHeader(header); err != nil {
		t.Fatalf("ar member %s: %v", name, err)
	}
	if _, err := writer.Write(body); err != nil {
		t.Fatalf("ar body %s: %v", name, err)
	}
}

const helloScript = "#!/bin/sh\necho hello\n"

func md5Hex(body string) string {
	sum := md5.Sum([]byte(body))
	return hex.EncodeToString(sum[:])
}

const sampleControl = `Package: hello-world
Version: 1:2.10-3
Architecture: amd64
Maintainer: Example Developer <dev@example.org>
Installed-Size: 128
Depends: libc6 (>= 2.34), libgtk-3-0 (>= 3.24.0), python3 | python3-minimal, apt
Recommends: libnotify4
Conflicts: hello-old
Section: utils
Priority: optional
Homepage: https://example.org/hello
Description: Example greeting program
 A longer description spanning
 several lines.
`

// sampleDeb builds a complete, valid .deb fixture using gzip members.
func sampleDeb(t *testing.T) string {
	t.Helper()
	control := buildTar(t, []tarFile{
		{Name: "./control", Body: sampleControl},
		{Name: "./md5sums", Body: fmt.Sprintf("%s usr/bin/hello-world\n", md5Hex(helloScript))},
		{Name: "./postinst", Body: "#!/bin/sh\nupdate-desktop-database || true\n", Mode: 0o755},
	})
	data := buildTar(t, []tarFile{
		{Name: "./usr/", Dir: true},
		{Name: "./usr/bin/", Dir: true},
		{Name: "./usr/bin/hello-world", Body: helloScript, Mode: 0o755},
		{Name: "./usr/share/applications/hello-world.desktop", Body: "[Desktop Entry]\nType=Application\nName=Hello\nExec=/usr/bin/hello-world\n"},
		{Name: "./usr/share/icons/hicolor/128x128/apps/hello-world.png", Body: "PNG"},
		{Name: "./usr/share/doc/hello-world/copyright", Body: "License: GPL-3\n"},
	})
	return writeDeb(t, debFixture{
		ControlName: "control.tar.gz",
		ControlData: compressGzip(t, control),
		DataName:    "data.tar.gz",
		DataData:    compressGzip(t, data),
	})
}
