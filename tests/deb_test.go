package tests

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Serge-Nook/kuznica/internal/deb"
)

func TestOpenReadsMetadata(t *testing.T) {
	pkg, err := deb.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pkg.Cleanup()

	if pkg.Name != "hello-world" {
		t.Errorf("name = %q", pkg.Name)
	}
	if pkg.Version != "1:2.10-3" {
		t.Errorf("version = %q", pkg.Version)
	}
	if got := pkg.UpstreamVersion(); got != "2.10" {
		t.Errorf("upstream version = %q, want 2.10", got)
	}
	if pkg.Architecture != "amd64" {
		t.Errorf("architecture = %q", pkg.Architecture)
	}
	if pkg.Homepage != "https://example.org/hello" {
		t.Errorf("homepage = %q", pkg.Homepage)
	}
	if pkg.InstalledBytes() != 128*1024 {
		t.Errorf("installed size = %d", pkg.InstalledBytes())
	}
	if pkg.License != "GPL-3" {
		t.Errorf("license = %q, want GPL-3", pkg.License)
	}
	if len(pkg.Depends) != 4 {
		t.Errorf("depends = %v", pkg.Depends)
	}
	if want := "Example greeting program"; !strings.HasPrefix(pkg.Description, want) {
		t.Errorf("description = %q", pkg.Description)
	}
	if !strings.Contains(pkg.Description, "several lines.") {
		t.Errorf("continuation lines lost: %q", pkg.Description)
	}
	if _, ok := pkg.Scripts["postinst"]; !ok {
		t.Errorf("postinst script not collected")
	}
	if got := pkg.Executables(); len(got) != 1 || got[0] != "usr/bin/hello-world" {
		t.Errorf("executables = %v", got)
	}
}

func TestVerifyChecksums(t *testing.T) {
	pkg, err := deb.Open(sampleDeb(t))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer pkg.Cleanup()

	mismatched, err := pkg.VerifyChecksums()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(mismatched) != 0 {
		t.Fatalf("unexpected mismatches: %v", mismatched)
	}

	target := filepath.Join(pkg.DataDir(), "usr", "bin", "hello-world")
	if err := os.WriteFile(target, []byte("tampered"), 0o755); err != nil {
		t.Fatalf("tamper: %v", err)
	}
	mismatched, err = pkg.VerifyChecksums()
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(mismatched) != 1 {
		t.Fatalf("tampered file not detected: %v", mismatched)
	}
}

func TestCompressionFormats(t *testing.T) {
	control := buildTar(t, []tarFile{{Name: "./control", Body: sampleControl}})
	data := buildTar(t, []tarFile{{Name: "./usr/bin/hello-world", Body: helloScript, Mode: 0o755}})

	cases := []struct {
		suffix   string
		compress func(*testing.T, []byte) []byte
	}{
		{".gz", compressGzip},
		{".xz", compressXZ},
		{".zst", compressZstd},
		{"", func(_ *testing.T, data []byte) []byte { return data }},
	}
	for _, tc := range cases {
		t.Run("data.tar"+tc.suffix, func(t *testing.T) {
			path := writeDeb(t, debFixture{
				ControlName: "control.tar" + tc.suffix,
				ControlData: tc.compress(t, control),
				DataName:    "data.tar" + tc.suffix,
				DataData:    tc.compress(t, data),
			})
			pkg, err := deb.Open(path)
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			defer pkg.Cleanup()
			if pkg.Name != "hello-world" {
				t.Errorf("name = %q", pkg.Name)
			}
		})
	}
}

func TestRejectsPathTraversal(t *testing.T) {
	control := buildTar(t, []tarFile{{Name: "./control", Body: sampleControl}})
	data := buildTar(t, []tarFile{{Name: "../../etc/passwd", Body: "root::0:0::/root:/bin/sh\n"}})
	path := writeDeb(t, debFixture{
		ControlName: "control.tar.gz",
		ControlData: compressGzip(t, control),
		DataName:    "data.tar.gz",
		DataData:    compressGzip(t, data),
	})

	if _, err := deb.Open(path); !errors.Is(err, deb.ErrPathTraversal) {
		t.Fatalf("error = %v, want ErrPathTraversal", err)
	}
}

func TestMissingMembers(t *testing.T) {
	control := compressGzip(t, buildTar(t, []tarFile{{Name: "./control", Body: sampleControl}}))
	data := compressGzip(t, buildTar(t, []tarFile{{Name: "./usr/bin/hello", Body: helloScript}}))

	cases := []struct {
		name    string
		fixture debFixture
		want    error
	}{
		{
			name:    "no control.tar",
			fixture: debFixture{SkipControl: true, DataName: "data.tar.gz", DataData: data},
			want:    deb.ErrNoControlTar,
		},
		{
			name:    "no data.tar",
			fixture: debFixture{ControlName: "control.tar.gz", ControlData: control, SkipData: true},
			want:    deb.ErrNoDataTar,
		},
		{
			name: "unsupported format version",
			fixture: debFixture{
				DebianBinary: "3.0\n",
				ControlName:  "control.tar.gz", ControlData: control,
				DataName: "data.tar.gz", DataData: data,
			},
			want: deb.ErrNotDebPackage,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := deb.Open(writeDeb(t, tc.fixture)); !errors.Is(err, tc.want) {
				t.Fatalf("error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestOpenRejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "broken.deb")
	if err := os.WriteFile(path, []byte("this is not an ar archive"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := deb.Open(path); err == nil {
		t.Fatal("expected an error for a corrupted package")
	}
}

func TestHumanSize(t *testing.T) {
	cases := map[int64]string{
		512:                    "512 B",
		2048:                   "2.0 KiB",
		5 * 1024 * 1024:        "5.0 MiB",
		3 * 1024 * 1024 * 1024: "3.0 GiB",
	}
	for size, want := range cases {
		if got := deb.HumanSize(size); got != want {
			t.Errorf("HumanSize(%d) = %q, want %q", size, got, want)
		}
	}
}
