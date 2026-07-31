package deb

import (
	"bufio"
	"strings"
)

// Control holds the fields of a Debian control file in their original order.
type Control struct {
	Fields map[string]string
	Order  []string
}

// ParseControl parses the RFC-822 style Debian control file, honouring
// continuation lines.
func ParseControl(text string) Control {
	ctrl := Control{Fields: map[string]string{}}
	scanner := bufio.NewScanner(strings.NewReader(text))
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	current := ""
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if current == "" {
				continue
			}
			value := strings.TrimSpace(line)
			if value == "." {
				value = ""
			}
			ctrl.Fields[current] += "\n" + value
			continue
		}
		key, value, found := strings.Cut(line, ":")
		if !found {
			continue
		}
		current = strings.ToLower(strings.TrimSpace(key))
		if _, exists := ctrl.Fields[current]; !exists {
			ctrl.Order = append(ctrl.Order, current)
		}
		ctrl.Fields[current] = strings.TrimSpace(value)
	}
	return ctrl
}

// Get returns a field value, ignoring case.
func (c Control) Get(name string) string {
	return c.Fields[strings.ToLower(name)]
}

// ParseMD5Sums parses a control archive md5sums file into a
// path → checksum map.
func ParseMD5Sums(text string) map[string]string {
	sums := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(text))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		sum, path, found := strings.Cut(line, " ")
		if !found {
			continue
		}
		sums[normalizeMemberName(strings.TrimSpace(path))] = sum
	}
	return sums
}

// SplitDependencies splits a Debian dependency field into its comma
// separated expressions.
func SplitDependencies(field string) []string {
	var out []string
	for _, part := range strings.Split(field, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

// ShortDescription returns the first line of a Debian description field.
func ShortDescription(description string) string {
	line, _, _ := strings.Cut(description, "\n")
	return strings.TrimSpace(line)
}
