// Package handoff builds the payload the skill-updater skill consumes on its
// programmatic path.
//
// Nothing here pushes anywhere. The payload is a pure function of threads already
// written to disk, so a failed handoff loses no work — the same call repeated gives
// the same result.
package handoff

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
)

const (
	defaultPriority = "medium"
	defaultCategory = "instructions"

	// A highlight can cover a whole code block, and skill-updater does not need it
	// repeated in full.
	quoteLimit = 200
)

var (
	priorities = []string{"high", "medium", "low"}
	categories = []string{"instructions", "tools", "examples", "error_handling", "structure", "references"}
)

type Suggestion struct {
	Priority       string `json:"priority"`
	Category       string `json:"category"`
	Suggestion     string `json:"suggestion"`
	ExpectedImpact string `json:"expected_impact"`
}

type Payload struct {
	SkillName   string       `json:"skill_name"`
	SkillPath   string       `json:"skill_path"`
	Suggestions []Suggestion `json:"improvement_suggestions"`
}

// Resolved threads and threads whose comments have all been retracted are left out.
func Build(threads []comments.Thread, skillName, skillPath string) Payload {
	payload := Payload{
		SkillName:   skillName,
		SkillPath:   skillPath,
		Suggestions: []Suggestion{},
	}

	for _, t := range threads {
		if t.Status == "resolved" {
			continue
		}
		bodies := liveBodies(t.Comments)
		if len(bodies) == 0 {
			continue
		}
		payload.Suggestions = append(payload.Suggestions, Suggestion{
			Priority:       oneOf(t.Priority, priorities, defaultPriority),
			Category:       oneOf(t.Category, categories, defaultCategory),
			Suggestion:     describe(bodies, t.Quote),
			ExpectedImpact: impact(t),
		})
	}

	slices.SortStableFunc(payload.Suggestions, func(a, b Suggestion) int {
		return slices.Index(priorities, a.Priority) - slices.Index(priorities, b.Priority)
	})
	return payload
}

var frontmatterName = regexp.MustCompile(`(?m)^name:[ \t]*["']?([^"'\r\n]+?)["']?[ \t]*$`)

// A YAML parser would be a dependency earned by exactly one field.
func SkillName(src []byte) string {
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

func liveBodies(cs []comments.Comment) []string {
	var bodies []string
	for _, c := range cs {
		if body := strings.TrimSpace(c.Body); !c.Deleted && body != "" {
			bodies = append(bodies, body)
		}
	}
	return bodies
}

func describe(bodies []string, quote string) string {
	note := strings.Join(bodies, "\n\n")
	if quote == "" {
		return note
	}
	return fmt.Sprintf("%s\n\nAnchored to: %q", note, truncate(quote))
}

func impact(t comments.Thread) string {
	if given := strings.TrimSpace(t.Impact); given != "" {
		return given
	}
	return fmt.Sprintf("Addresses reviewer feedback on %q.", truncate(t.Quote))
}

func truncate(s string) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= quoteLimit {
		return s
	}
	return string(runes[:quoteLimit]) + "…"
}

func oneOf(value string, allowed []string, fallback string) string {
	if slices.Contains(allowed, value) {
		return value
	}
	return fallback
}
