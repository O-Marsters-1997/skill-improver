// Package skill reads what a SKILL.md says about itself.
//
// It exists as its own package because two callers need it and neither can import the
// other: handoff names the skill under review, config names the skill that applies the
// suggestions.
package skill

import (
	"regexp"
	"strings"
)

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
