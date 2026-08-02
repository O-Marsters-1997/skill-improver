// Package handoff builds the payload the updater skill consumes on its programmatic
// path.
//
// Build pushes nowhere: the payload is a pure function of threads already written to
// disk, so a failed handoff loses no work and the same call repeated gives the same
// result. Submit, in pending.go, is the one thing here that touches the filesystem.
package handoff

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
	"github.com/O-Marsters-1997/improve-skills/internal/config"
)

// A highlight can cover a whole code block, and the updater does not need it repeated
// in full.
const quoteLimit = 200

// ID is the originating thread's, and is what tells a later handoff that this
// suggestion has already been applied. File is the absolute path of the document the
// thread was anchored in, which is not the skill when the review is of the skill's
// output. Fields carries whatever the config asks for, and is written flat alongside
// the rest.
type Suggestion struct {
	ID             string
	File           string
	Fields         map[string]string
	Suggestion     string
	ExpectedImpact string
}

// Every value in a suggestion is a string, so one map is the whole object; encoding/json
// sorts its keys, which is what keeps an unchanged payload comparing equal.
func (s Suggestion) MarshalJSON() ([]byte, error) {
	flat := map[string]string{
		"id":              s.ID,
		"file":            s.File,
		"suggestion":      s.Suggestion,
		"expected_impact": s.ExpectedImpact,
	}
	maps.Copy(flat, s.Fields)
	return json.Marshal(flat)
}

func (s *Suggestion) UnmarshalJSON(data []byte) error {
	var flat map[string]string
	if err := json.Unmarshal(data, &flat); err != nil {
		return err
	}
	s.ID, s.File = flat["id"], flat["file"]
	s.Suggestion, s.ExpectedImpact = flat["suggestion"], flat["expected_impact"]

	delete(flat, "id")
	delete(flat, "file")
	delete(flat, "suggestion")
	delete(flat, "expected_impact")
	if len(flat) > 0 {
		s.Fields = flat
	}
	return nil
}

// Mode tells the updater what it is being shown. It is derived from the paths rather
// than declared, because a flag would let the two disagree.
const (
	// ModeInstructions — the reviewed document is inside the skill, so a suggestion is
	// about the text it names.
	ModeInstructions = "instructions"
	// ModeOutput — the reviewed document is something the skill produced, so the
	// instruction that caused a suggestion has to be inferred.
	ModeOutput = "output"
)

type Payload struct {
	SkillName   string       `json:"skill_name"`
	SkillPath   string       `json:"skill_path"`
	Mode        string       `json:"mode"`
	Suggestions []Suggestion `json:"improvement_suggestions"`
}

// Build turns one document's threads into a payload. Resolved threads and threads whose
// comments have all been retracted are left out.
//
// Submit is what completes the result: Mode and each Suggestion's File depend on the
// paths of the review, which Build is not given.
func Build(cfg *config.Config, threads []comments.Thread, skillName, skillPath string) Payload {
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

		fields := make(map[string]string, len(cfg.Fields))
		for _, f := range cfg.Fields {
			fields[f.Name] = oneOf(t.Fields[f.Name], f.Values, f.Default)
		}
		payload.Suggestions = append(payload.Suggestions, Suggestion{
			ID:             t.ID,
			Fields:         fields,
			Suggestion:     describe(bodies, t.Quote),
			ExpectedImpact: impact(t),
		})
	}

	sortSuggestions(cfg, payload.Suggestions)
	return payload
}

// The first field is the ranking one, ordered as its values are listed. Submit runs this
// again over the concatenation of several documents' suggestions, which a stable sort
// leaves ranked across the whole review rather than within each file.
func sortSuggestions(cfg *config.Config, suggestions []Suggestion) {
	sortBy, ok := cfg.SortField()
	if !ok {
		return
	}
	slices.SortStableFunc(suggestions, func(a, b Suggestion) int {
		return slices.Index(sortBy.Values, a.Fields[sortBy.Name]) -
			slices.Index(sortBy.Values, b.Fields[sortBy.Name])
	})
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
