// Package config stores the persistent user preferences of КУЗНИЦА.
package config

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Theme string

const (
	ThemeSystem Theme = "system"
	ThemeLight  Theme = "light"
	ThemeDark   Theme = "dark"
)

type Config struct {
	Theme            Theme   `json:"theme"`
	Language         string  `json:"language"`
	UseMakepkg       bool    `json:"use_makepkg"`
	RemoveTempFiles  bool    `json:"remove_temp_files"`
	AutoInstallDeps  bool    `json:"auto_install_deps"`
	CreateDesktop    bool    `json:"create_desktop"`
	ValidateDesktop  bool    `json:"validate_desktop"`
	AutoDetectIcons  bool    `json:"auto_detect_icons"`
	ShowLogAfterMake bool    `json:"show_log_after_make"`
	OutputDir        string  `json:"output_dir"`
	UIScale          float64 `json:"ui_scale"`
}

// Interface scale factors relative to the compact 1280x800 layout.
const (
	ScaleCompact = 1.0
	ScaleNormal  = 1.15
	ScaleLarge   = 1.3
)

// Default returns the configuration used on the first run.
func Default() Config {
	home, err := os.UserHomeDir()
	if err != nil {
		home = os.TempDir()
	}
	return Config{
		Theme:            ThemeSystem,
		Language:         "ru",
		UseMakepkg:       true,
		RemoveTempFiles:  true,
		AutoInstallDeps:  false,
		CreateDesktop:    true,
		ValidateDesktop:  true,
		AutoDetectIcons:  true,
		ShowLogAfterMake: true,
		OutputDir:        filepath.Join(home, "kuznica"),
		UIScale:          ScaleCompact,
	}
}

// Path returns the location of the configuration file.
func Path() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = os.TempDir()
	}
	return filepath.Join(dir, "kuznica", "config.json")
}

// Load reads the configuration from path, falling back to the defaults when
// the file does not exist yet.
func Load(path string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), err
	}
	cfg.normalize()
	return cfg, nil
}

// Save writes the configuration to path, creating parent directories.
func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func (c *Config) normalize() {
	def := Default()
	switch c.Theme {
	case ThemeSystem, ThemeLight, ThemeDark:
	default:
		c.Theme = def.Theme
	}
	if c.Language != "ru" && c.Language != "en" {
		c.Language = def.Language
	}
	if c.OutputDir == "" {
		c.OutputDir = def.OutputDir
	}
	c.UIScale = c.NormalizedUIScale()
}

// NormalizedUIScale clamps the interface scale to the supported range.
func (c Config) NormalizedUIScale() float64 {
	switch {
	case c.UIScale < ScaleCompact:
		return ScaleCompact
	case c.UIScale > ScaleLarge:
		return ScaleLarge
	default:
		return c.UIScale
	}
}
