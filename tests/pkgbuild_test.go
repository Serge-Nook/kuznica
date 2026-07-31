package tests

import (
	"strings"
	"testing"

	"github.com/Serge-Nook/kuznica/internal/pkgbuild"
)

func sampleSpec() pkgbuild.Spec {
	return pkgbuild.Spec{
		PkgName:    "Hello World",
		PkgVersion: "2.10",
		PkgRelease: "1",
		PkgDesc:    "Example greeting program\nwith a second line",
		Arch:       pkgbuild.ArchFromDebian("amd64"),
		URL:        "https://example.org/hello",
		License:    []string{"GPL-3"},
		Depends:    []string{"glibc>=2.34", "gtk3"},
		OptDepends: []string{"libnotify"},
		Conflicts:  []string{"hello-old"},
		Maintainer: "Example Developer <dev@example.org>",
	}
}

func TestRenderPKGBUILD(t *testing.T) {
	rendered := sampleSpec().Render()
	for _, want := range []string{
		"pkgname=hello-world",
		"pkgver=2.10",
		"pkgrel=1",
		"arch=('x86_64')",
		"url='https://example.org/hello'",
		"license=('GPL-3')",
		"depends=('glibc>=2.34' 'gtk3')",
		"optdepends=('libnotify')",
		"conflicts=('hello-old')",
		"package() {",
	} {
		if !strings.Contains(rendered, want) {
			t.Errorf("PKGBUILD does not contain %q:\n%s", want, rendered)
		}
	}
	if strings.Contains(rendered, "with a second line\n") {
		t.Error("pkgdesc must be a single line")
	}
}

func TestRenderSRCINFO(t *testing.T) {
	rendered := sampleSpec().RenderSRCINFO()
	for _, want := range []string{"pkgbase = hello-world", "pkgver = 2.10", "arch = x86_64", "depends = gtk3"} {
		if !strings.Contains(rendered, want) {
			t.Errorf(".SRCINFO does not contain %q:\n%s", want, rendered)
		}
	}
}

func TestRenderInstallHook(t *testing.T) {
	spec := sampleSpec()
	if spec.NeedsInstallHook() {
		t.Fatal("no maintainer scripts, hook should not be needed")
	}
	spec.PostInstall = "update-desktop-database || true"
	if !spec.NeedsInstallHook() {
		t.Fatal("hook expected once a script is present")
	}
	hook := spec.RenderInstall()
	for _, want := range []string{"post_install() {", "post_upgrade() {", "update-desktop-database"} {
		if !strings.Contains(hook, want) {
			t.Errorf(".INSTALL does not contain %q:\n%s", want, hook)
		}
	}
	if !strings.Contains(spec.Render(), "install=hello-world.install") {
		t.Error("PKGBUILD must reference the install hook")
	}
}

func TestSanitizers(t *testing.T) {
	names := map[string]string{
		"Hello World": "hello-world",
		"lib++":       "lib++",
		"--weird--":   "weird--",
		"":            "converted-package",
	}
	for input, want := range names {
		if got := pkgbuild.SanitizeName(input); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", input, got, want)
		}
	}
	versions := map[string]string{
		"2.10":     "2.10",
		"1:2.10-3": "1.2.10.3",
		"":         "0",
	}
	for input, want := range versions {
		if got := pkgbuild.SanitizeVersion(input); got != want {
			t.Errorf("SanitizeVersion(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestArchFromDebian(t *testing.T) {
	cases := map[string]string{
		"amd64": "x86_64",
		"arm64": "aarch64",
		"i386":  "i686",
		"all":   "any",
		"armhf": "armv7h",
	}
	for input, want := range cases {
		got := pkgbuild.ArchFromDebian(input)
		if len(got) != 1 || got[0] != want {
			t.Errorf("ArchFromDebian(%q) = %v, want [%s]", input, got, want)
		}
	}
}
