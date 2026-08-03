// Package steam adapts converted packages for the SteamOS game mode: it
// installs them into a writable prefix and registers them in Steam as
// non-Steam shortcuts, which is the only way the gamescope session can
// launch a program.
package steam

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Binary VDF markers as used by Steam.
const (
	typeMap    byte = 0x00
	typeString byte = 0x01
	typeInt32  byte = 0x02
	typeEnd    byte = 0x08
)

// ErrMalformedVDF is returned for a shortcuts.vdf KUZNICA cannot parse.
var ErrMalformedVDF = errors.New("malformed binary VDF")

// Map is an ordered VDF key/value collection. The order is preserved so a
// rewritten shortcuts.vdf stays as close to the original as possible.
type Map struct {
	keys   []string
	values map[string]any // string, int32 or *Map
}

// NewMap creates an empty map.
func NewMap() *Map {
	return &Map{values: map[string]any{}}
}

// Len returns the number of entries.
func (m *Map) Len() int { return len(m.keys) }

// Keys returns the keys in insertion order.
func (m *Map) Keys() []string { return append([]string(nil), m.keys...) }

// Get returns the value stored under key.
func (m *Map) Get(key string) (any, bool) {
	value, ok := m.values[key]
	return value, ok
}

// String returns the string stored under key, or an empty string.
func (m *Map) String(key string) string {
	if value, ok := m.values[key].(string); ok {
		return value
	}
	return ""
}

// Child returns the nested map stored under key.
func (m *Map) Child(key string) (*Map, bool) {
	child, ok := m.values[key].(*Map)
	return child, ok
}

// Set stores a value, replacing an existing entry in place.
func (m *Map) Set(key string, value any) {
	if _, exists := m.values[key]; !exists {
		m.keys = append(m.keys, key)
	}
	m.values[key] = value
}

// Delete removes an entry.
func (m *Map) Delete(key string) {
	if _, exists := m.values[key]; !exists {
		return
	}
	delete(m.values, key)
	for i, existing := range m.keys {
		if existing == key {
			m.keys = append(m.keys[:i], m.keys[i+1:]...)
			break
		}
	}
}

// ReadVDF parses a binary VDF document.
func ReadVDF(r io.Reader) (*Map, error) {
	reader := bufio.NewReader(r)
	root := NewMap()
	if err := readInto(reader, root); err != nil {
		return nil, err
	}
	return root, nil
}

func readInto(r *bufio.Reader, target *Map) error {
	for {
		marker, err := r.ReadByte()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if marker == typeEnd {
			return nil
		}
		key, err := readString(r)
		if err != nil {
			return err
		}
		switch marker {
		case typeMap:
			child := NewMap()
			if err := readInto(r, child); err != nil {
				return err
			}
			target.Set(key, child)
		case typeString:
			value, err := readString(r)
			if err != nil {
				return err
			}
			target.Set(key, value)
		case typeInt32:
			var value int32
			if err := binary.Read(r, binary.LittleEndian, &value); err != nil {
				return err
			}
			target.Set(key, value)
		default:
			return fmt.Errorf("%w: unsupported entry type 0x%02x", ErrMalformedVDF, marker)
		}
	}
}

func readString(r *bufio.Reader) (string, error) {
	value, err := r.ReadString(0)
	if err != nil {
		return "", err
	}
	return strings.TrimSuffix(value, "\x00"), nil
}

// WriteVDF serialises a document in the binary form Steam expects: every
// nested map is terminated by 0x08 and the document itself by one more 0x08.
func WriteVDF(w io.Writer, root *Map) error {
	writer := bufio.NewWriter(w)
	if err := writeMap(writer, root); err != nil {
		return err
	}
	if err := writer.WriteByte(typeEnd); err != nil {
		return err
	}
	return writer.Flush()
}

func writeMap(w *bufio.Writer, m *Map) error {
	for _, key := range m.keys {
		switch value := m.values[key].(type) {
		case *Map:
			if err := writeHeader(w, typeMap, key); err != nil {
				return err
			}
			if err := writeMap(w, value); err != nil {
				return err
			}
			if err := w.WriteByte(typeEnd); err != nil {
				return err
			}
		case string:
			if err := writeHeader(w, typeString, key); err != nil {
				return err
			}
			if err := writeString(w, value); err != nil {
				return err
			}
		case int32:
			if err := writeHeader(w, typeInt32, key); err != nil {
				return err
			}
			if err := binary.Write(w, binary.LittleEndian, value); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%w: cannot write %T", ErrMalformedVDF, value)
		}
	}
	return nil
}

func writeHeader(w *bufio.Writer, marker byte, key string) error {
	if err := w.WriteByte(marker); err != nil {
		return err
	}
	return writeString(w, key)
}

func writeString(w *bufio.Writer, value string) error {
	if strings.ContainsRune(value, 0) {
		return fmt.Errorf("%w: NUL byte in %q", ErrMalformedVDF, value)
	}
	if _, err := w.WriteString(value); err != nil {
		return err
	}
	return w.WriteByte(0)
}
