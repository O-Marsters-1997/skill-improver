// Package skill reads what a SKILL.md says about itself.
//
// It exists as its own package because two callers need it and neither can import the
// other: handoff names the skill under review, config names the skill that applies the
// suggestions.
package skill

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// FileName is the one file a skill directory is required to have.
const FileName = "SKILL.md"

// Resolve turns a skill path — either a directory or the SKILL.md itself — into the
// absolute path of its SKILL.md. Symlinks are left alone: mode derivation resolves them
// on both sides, and the payload should name the path the reviewer gave.
func Resolve(path string) (skillMD string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("skill: resolve %s: %w", path, err)
	}
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return filepath.Join(abs, FileName), nil
	}
	return abs, nil
}

// NameAt is Name against a path rather than bytes, for the callers that hold a skill
// rather than a document: an unreadable file is nameless, not an error.
func NameAt(path string) string {
	skillMD, err := Resolve(path)
	if err != nil {
		return ""
	}
	src, err := os.ReadFile(skillMD)
	if err != nil {
		return ""
	}
	return Name(src)
}

var frontmatterName = regexp.MustCompile(`(?m)^name:[ \t]*["']?([^"'\r\n]+?)["']?[ \t]*$`)

// A YAML parser would be a dependency earned by exactly one field.
func Name(src []byte) string {
	const delimiter = "---\n"
	body, ok := strings.CutPrefix(string(src), delimiter)
	if !ok {
		return ""
	}
	block, _, ok := strings.Cut(body, "\n"+delimiter)
	if !ok {
		return ""
	}
	m := frontmatterName.FindStringSubmatch(block)
	if m == nil {
		return ""
	}
	return m[1]
}
