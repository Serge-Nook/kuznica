package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Serge-Nook/kuznica/internal/mapping"
)

func TestTranslateKnownPackages(t *testing.T) {
	db, err := mapping.NewDefault()
	if err != nil {
		t.Fatalf("load database: %v", err)
	}
	cases := map[string]string{
		"libc6":        "glibc",
		"python3":      "python",
		"libgtk-3-0":   "gtk3",
		"libssl3":      "openssl",
		"libnotify4":   "libnotify",
		"libqt5core5a": "qt5-base",
	}
	for debian, want := range cases {
		if got := db.Translate(debian); got.Arch != want {
			t.Errorf("Translate(%q).Arch = %q, want %q", debian, got.Arch, want)
		}
	}
}

func TestTranslateVersionConstraint(t *testing.T) {
	db, _ := mapping.NewDefault()
	res := db.Translate("libc6 (>= 2.34-1ubuntu1)")
	if res.Arch != "glibc>=2.34" {
		t.Errorf("Arch = %q, want glibc>=2.34", res.Arch)
	}
	if res.Constraint != ">=2.34" {
		t.Errorf("Constraint = %q", res.Constraint)
	}
}

func TestTranslateAlternatives(t *testing.T) {
	db, _ := mapping.NewDefault()
	res := db.Translate("python3-minimal | python3")
	if res.Arch != "python" {
		t.Errorf("Arch = %q, want python", res.Arch)
	}
	if !res.Optional {
		t.Error("alternatives should be marked optional")
	}
}

func TestIgnoredDependencies(t *testing.T) {
	db, _ := mapping.NewDefault()
	for _, name := range []string{"dpkg", "debconf", "install-info"} {
		if res := db.Translate(name); !res.Ignored {
			t.Errorf("%q should be ignored, got %+v", name, res)
		}
	}
	deps := db.ArchDepends("libc6, dpkg, libgtk-3-0")
	if len(deps) != 2 {
		t.Fatalf("ArchDepends = %v, want two entries", deps)
	}
}

func TestUnknownPackageIsGuessed(t *testing.T) {
	db, _ := mapping.NewDefault()
	cases := map[string]string{
		"libfoobar7":      "libfoobar",
		"libwhatever-dev": "libwhatever",
		"somecli":         "somecli",
	}
	for debian, want := range cases {
		res := db.Translate(debian)
		if res.Mapped {
			t.Errorf("%q unexpectedly present in the database", debian)
		}
		if res.Arch != want {
			t.Errorf("Translate(%q).Arch = %q, want %q", debian, res.Arch, want)
		}
	}
}

func TestUserDatabaseOverridesDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "mappings.json")
	body := `{"packages": {"libc6": "glibc-custom", "mytool": "mytool-git"}}`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	db, err := mapping.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := db.Translate("libc6").Arch; got != "glibc-custom" {
		t.Errorf("override ignored: %q", got)
	}
	if got := db.Translate("mytool").Arch; got != "mytool-git" {
		t.Errorf("user entry ignored: %q", got)
	}
}

func TestSaveRoundTrip(t *testing.T) {
	db, _ := mapping.NewDefault()
	db.Set("example-lib", "example")
	path := filepath.Join(t.TempDir(), "mappings.json")
	if err := db.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded, err := mapping.Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got := reloaded.Translate("example-lib").Arch; got != "example" {
		t.Errorf("round trip lost the entry: %q", got)
	}
}
