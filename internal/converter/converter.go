// Package converter orchestrates the whole Debian → Arch pipeline: package
// analysis, dependency translation, PKGBUILD generation, makepkg build and
// pacman installation.
package converter

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Serge-Nook/kuznica/internal/config"
	"github.com/Serge-Nook/kuznica/internal/deb"
	"github.com/Serge-Nook/kuznica/internal/desktop"
	"github.com/Serge-Nook/kuznica/internal/installer"
	"github.com/Serge-Nook/kuznica/internal/logger"
	"github.com/Serge-Nook/kuznica/internal/mapping"
	"github.com/Serge-Nook/kuznica/internal/packager"
	"github.com/Serge-Nook/kuznica/internal/pkgbuild"
)

// ErrNoPackage is returned when a stage is executed before a package has
// been opened.
var ErrNoPackage = errors.New("no package has been opened")

// Conversion is the state of a single Debian → Arch conversion.
type Conversion struct {
	Package      *deb.Package
	Spec         pkgbuild.Spec
	Dependencies []mapping.Result
	BuildDir     string
	PKGBUILDPath string
	SRCINFOPath  string
	InstallPath  string
	DesktopPath  string
	DesktopEntry desktop.Entry
	ArtifactPath string
}

// Converter turns Debian packages into Arch Linux packages.
type Converter struct {
	cfg      config.Config
	log      *logger.Logger
	mappings *mapping.Database
}

// New creates a converter using the given configuration and mapping database.
func New(cfg config.Config, log *logger.Logger, mappings *mapping.Database) *Converter {
	return &Converter{cfg: cfg, log: log, mappings: mappings}
}

// SetConfig replaces the configuration used by the following stages.
func (c *Converter) SetConfig(cfg config.Config) { c.cfg = cfg }

// Config returns the active configuration.
func (c *Converter) Config() config.Config { return c.cfg }

// Mappings exposes the dependency database.
func (c *Converter) Mappings() *mapping.Database { return c.mappings }

// Open analyses a .deb file and returns a conversion in its initial state.
func (c *Converter) Open(path string) (*Conversion, error) {
	c.log.Infof("Opening package: %s", path)
	if !strings.HasSuffix(strings.ToLower(path), ".deb") {
		c.log.Warningf("File %s does not have the .deb extension", filepath.Base(path))
	}

	pkg, err := deb.Open(path)
	if err != nil {
		c.log.Errorf("Failed to open %s: %v", filepath.Base(path), err)
		return nil, err
	}
	c.log.Infof("Package %s %s (%s), %d files", pkg.Name, pkg.Version, pkg.Architecture, len(pkg.Files))

	mismatched, err := pkg.VerifyChecksums()
	if err != nil {
		c.log.Errorf("Checksum verification failed: %v", err)
	} else if len(mismatched) > 0 {
		c.log.Warningf("Checksum mismatch for %d file(s), first: %s", len(mismatched), mismatched[0])
	} else if len(pkg.Checksums) > 0 {
		c.log.Infof("All %d checksums verified", len(pkg.Checksums))
	}

	return &Conversion{Package: pkg}, nil
}

// Convert translates the metadata and dependencies and renders the makepkg
// input files into the build directory.
func (c *Converter) Convert(conv *Conversion) error {
	if conv == nil || conv.Package == nil {
		return ErrNoPackage
	}
	pkg := conv.Package
	c.log.Infof("Converting %s to an Arch Linux package", pkg.Name)

	conv.Dependencies = c.translateDependencies(pkg)

	depends := uniqueArch(conv.Dependencies)
	optDepends := c.mappings.ArchDepends(strings.Join(pkg.Recommends, ", "))
	conflicts := c.mappings.ArchDepends(strings.Join(append(append([]string{}, pkg.Conflicts...), pkg.Breaks...), ", "))
	replaces := c.mappings.ArchDepends(strings.Join(pkg.Replaces, ", "))
	provides := c.mappings.ArchDepends(strings.Join(pkg.Provides, ", "))

	conv.Spec = pkgbuild.Spec{
		PkgName:     pkgbuild.SanitizeName(pkg.Name),
		PkgVersion:  pkgbuild.SanitizeVersion(pkg.UpstreamVersion()),
		PkgRelease:  "1",
		PkgDesc:     deb.ShortDescription(pkg.Description),
		Arch:        pkgbuild.ArchFromDebian(pkg.Architecture),
		URL:         pkg.Homepage,
		License:     []string{defaultLicense(pkg.License)},
		Depends:     depends,
		OptDepends:  optDepends,
		Conflicts:   conflicts,
		Replaces:    replaces,
		Provides:    provides,
		Maintainer:  pkg.Maintainer,
		PreInstall:  pkg.Scripts["preinst"],
		PostInstall: pkg.Scripts["postinst"],
		PreRemove:   pkg.Scripts["prerm"],
		PostRemove:  pkg.Scripts["postrm"],
	}

	buildDir := filepath.Join(c.cfg.OutputDir, conv.Spec.PkgName)
	if err := os.MkdirAll(buildDir, 0o755); err != nil {
		return fmt.Errorf("create build directory: %w", err)
	}
	if err := installer.EnsureSpace(buildDir, pkg.InstalledBytes()*2+pkg.FileSize); err != nil {
		c.log.Errorf("%v", err)
		return err
	}
	conv.BuildDir = buildDir

	conv.PKGBUILDPath = filepath.Join(buildDir, "PKGBUILD")
	if err := os.WriteFile(conv.PKGBUILDPath, []byte(conv.Spec.Render()), 0o644); err != nil {
		return err
	}
	c.log.Infof("PKGBUILD written to %s", conv.PKGBUILDPath)

	conv.SRCINFOPath = filepath.Join(buildDir, ".SRCINFO")
	if err := os.WriteFile(conv.SRCINFOPath, []byte(conv.Spec.RenderSRCINFO()), 0o644); err != nil {
		return err
	}

	if hook := conv.Spec.RenderInstall(); hook != "" {
		conv.InstallPath = filepath.Join(buildDir, conv.Spec.PkgName+".install")
		if err := os.WriteFile(conv.InstallPath, []byte(hook), 0o644); err != nil {
			return err
		}
		c.log.Infof("Maintainer scripts converted into %s", filepath.Base(conv.InstallPath))
	}

	if err := c.stagePayload(conv); err != nil {
		return err
	}

	if c.cfg.CreateDesktop {
		if err := c.PrepareDesktopEntry(conv); err != nil {
			c.log.Warningf("Desktop entry generation failed: %v", err)
		}
	}
	c.log.Infof("Conversion of %s finished", pkg.Name)
	return nil
}

// stagePayload links or copies the extracted data.tar payload into the
// build directory where the generated PKGBUILD expects it.
func (c *Converter) stagePayload(conv *Conversion) error {
	payload := filepath.Join(conv.BuildDir, "payload")
	if err := os.RemoveAll(payload); err != nil {
		return err
	}
	if err := os.Symlink(conv.Package.DataDir(), payload); err != nil {
		return fmt.Errorf("stage payload: %w", err)
	}
	return nil
}

func (c *Converter) translateDependencies(pkg *deb.Package) []mapping.Result {
	all := append(append([]string{}, pkg.PreDepends...), pkg.Depends...)
	results := make([]mapping.Result, 0, len(all))
	for _, dep := range all {
		res := c.mappings.Translate(dep)
		switch {
		case res.Ignored:
			c.log.Debugf("Dependency %s is provided by the Arch base system, skipped", res.Debian)
		case res.Mapped:
			c.log.Infof("Dependency %s → %s", res.Debian, res.Arch)
		default:
			c.log.Warningf("Dependency %s has no mapping, guessed %s", res.Debian, res.Arch)
		}
		results = append(results, res)
	}
	return results
}

// PrepareDesktopEntry suggests a Desktop Entry for the package, reusing the
// one shipped inside the payload when present.
func (c *Converter) PrepareDesktopEntry(conv *Conversion) error {
	if conv == nil || conv.Package == nil {
		return ErrNoPackage
	}
	pkg := conv.Package
	existing := desktop.FindExisting(pkg.DataDir())
	if len(existing) > 0 {
		data, err := os.ReadFile(existing[0])
		if err == nil {
			conv.DesktopEntry = desktop.Parse(string(data))
			conv.DesktopPath = existing[0]
			c.log.Infof("Package already ships a desktop entry: %s", filepath.Base(existing[0]))
			return nil
		}
	}
	conv.DesktopEntry = desktop.Suggest(
		pkg.DataDir(),
		pkg.Name,
		deb.ShortDescription(pkg.Description),
		pkg.Section,
		pkg.Executables(),
		c.cfg.AutoDetectIcons,
	)
	conv.DesktopPath = filepath.Join(pkg.DataDir(), "usr", "share", "applications", pkgbuild.SanitizeName(pkg.Name)+".desktop")
	c.log.Infof("Desktop entry generated for %s", pkg.Name)
	return nil
}

// SaveDesktopEntry writes the entry into the payload and optionally
// validates it with desktop-file-validate.
func (c *Converter) SaveDesktopEntry(ctx context.Context, conv *Conversion, entry desktop.Entry) error {
	if conv == nil || conv.Package == nil {
		return ErrNoPackage
	}
	if conv.DesktopPath == "" {
		conv.DesktopPath = filepath.Join(conv.Package.DataDir(), "usr", "share", "applications",
			pkgbuild.SanitizeName(conv.Package.Name)+".desktop")
	}
	conv.DesktopEntry = entry
	if err := desktop.Write(conv.DesktopPath, entry); err != nil {
		c.log.Errorf("Cannot write desktop entry: %v", err)
		return err
	}
	c.log.Infof("Desktop entry saved: %s", conv.DesktopPath)

	if !c.cfg.ValidateDesktop {
		return nil
	}
	output, err := desktop.Validate(ctx, conv.DesktopPath)
	switch {
	case errors.Is(err, desktop.ErrValidatorMissing):
		c.log.Warningf("desktop-file-validate is not installed, validation skipped")
	case err != nil:
		c.log.Warningf("desktop-file-validate reported problems: %s", strings.TrimSpace(output))
	default:
		c.log.Infof("Desktop entry validated successfully")
	}
	return nil
}

// Build runs makepkg in the build directory and records the artifact path.
func (c *Converter) Build(ctx context.Context, conv *Conversion) error {
	if conv == nil || conv.BuildDir == "" {
		return errors.New("run the conversion before building")
	}
	if c.cfg.UseMakepkg {
		if missing := installer.MissingBuildTools(); len(missing) > 0 && c.cfg.AutoInstallDeps {
			c.log.Infof("Missing build tools (%s), installing base-devel with pacman", strings.Join(missing, ", "))
			if err := installer.InstallDependencies(ctx, []string{"base-devel"}, c.logLine); err != nil {
				c.log.Warningf("base-devel installation failed: %v", err)
			}
		}
	}
	if c.cfg.AutoInstallDeps && len(conv.Spec.Depends) > 0 {
		c.log.Infof("Installing %d dependencies with pacman", len(conv.Spec.Depends))
		if err := installer.InstallDependencies(ctx, conv.Spec.Depends, c.logLine); err != nil {
			c.log.Warningf("Dependency installation failed: %v", err)
		}
	}

	artifact, err := c.runBuild(ctx, conv)
	if err != nil {
		c.log.Errorf("Build failed: %v", err)
		return err
	}
	conv.ArtifactPath = artifact
	c.log.Infof("Package built: %s", artifact)
	return nil
}

// runBuild builds through makepkg when the Arch tool chain is present and
// falls back to the built-in packager otherwise (SteamOS, containers, and
// any system with a read-only root filesystem).
func (c *Converter) runBuild(ctx context.Context, conv *Conversion) (string, error) {
	missing := installer.MissingBuildTools()
	switch {
	case !c.cfg.UseMakepkg:
		c.log.Infof("makepkg is disabled in the settings, using the built-in packager")
	case len(missing) > 0:
		c.log.Warningf("Missing build tools: %s (package base-devel), using the built-in packager",
			strings.Join(missing, ", "))
	default:
		c.log.Infof("Running makepkg in %s", conv.BuildDir)
		return installer.Makepkg(ctx, conv.BuildDir, c.logLine)
	}

	builder := packager.Builder{
		Spec:       conv.Spec,
		PayloadDir: conv.Package.DataDir(),
		Install:    conv.Spec.RenderInstall(),
		OutputDir:  conv.BuildDir,
	}
	c.log.Infof("Packing %s without makepkg", conv.Spec.PkgName)
	return builder.Build()
}

// Install installs the built package through pacman.
func (c *Converter) Install(ctx context.Context, conv *Conversion) error {
	if conv == nil || conv.ArtifactPath == "" {
		return errors.New("build the package before installing")
	}
	c.log.Infof("Installing %s", filepath.Base(conv.ArtifactPath))
	if err := installer.Install(ctx, conv.ArtifactPath, c.logLine); err != nil {
		c.log.Errorf("Installation failed: %v", err)
		return err
	}
	c.log.Infof("Package installed successfully")
	return nil
}

// Cleanup removes the temporary files of a conversion when the setting is on.
func (c *Converter) Cleanup(conv *Conversion) {
	if conv == nil || conv.Package == nil || !c.cfg.RemoveTempFiles {
		return
	}
	if err := conv.Package.Cleanup(); err != nil {
		c.log.Warningf("Cannot remove temporary files: %v", err)
		return
	}
	c.log.Debugf("Temporary files removed")
}

func (c *Converter) logLine(line string) {
	if strings.TrimSpace(line) == "" {
		return
	}
	c.log.Debugf("%s", line)
}

func uniqueArch(results []mapping.Result) []string {
	seen := map[string]bool{}
	var out []string
	for _, res := range results {
		if res.Ignored || res.Arch == "" || seen[res.Arch] {
			continue
		}
		seen[res.Arch] = true
		out = append(out, res.Arch)
	}
	return out
}

func defaultLicense(license string) string {
	license = strings.TrimSpace(license)
	if license == "" {
		return "custom"
	}
	return license
}
