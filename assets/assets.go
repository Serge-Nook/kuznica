// Package assets embeds the static resources shipped with КУЗНИЦА.
package assets

import _ "embed"

// DefaultMappings is the built-in Debian ↔ Arch package correspondence
// database. Users may override entries through their own mappings.json.
//
//go:embed mappings.json
var DefaultMappings []byte

// AppIcon is the application icon used by the graphical interface.
//
//go:embed icons/kuznica.svg
var AppIcon []byte
