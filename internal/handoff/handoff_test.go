package handoff

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
	"github.com/O-Marsters-1997/improve-skills/internal/config"
)

func thread(id, priority string, bodies ...string) comments.Thread {
	t := comments.Thread{ID: id, Quote: "the anchored passage", Status: "open"}
	if priority != "" {
		t.Fields = map[string]string{"priority": priority}
	}
	for i, body := range bodies {
		t.Comments = append(t.Comments, comments.Comment{
			ID: string(rune('a' + i)), Author: "olly", Body: body,
		})
	}
	return t
}

func TestBuild(t *testing.T) {
	t.Run("orders by priority, then by document position", func(t *testing.T) {
		threads := []comments.Thread{
			thread("a", "low", "last"),
			thread("b", "high", "first"),
			thread("c", "medium", "middle"),
			thread("d", "high", "second"),
		}

		got := Build(config.Default(), threads, "example-skill", "/skills/example/SKILL.md")

		if got.SkillName != "example-skill" || got.SkillPath != "/skills/example/SKILL.md" {
			t.Errorf("payload header = %+v", got)
		}
		want := []string{"first", "second", "middle", "last"}
		if len(got.Suggestions) != len(want) {
			t.Fatalf("got %d suggestions; want %d", len(got.Suggestions), len(want))
		}
		for i, w := range want {
			if !strings.Contains(got.Suggestions[i].Suggestion, w) {
				t.Errorf("suggestion %d = %q; want it to contain %q", i, got.Suggestions[i].Suggestion, w)
			}
		}
	})

	t.Run("skips threads with nothing actionable", func(t *testing.T) {
		resolved := thread("a", "high", "already handled")
		resolved.Status = "resolved"

		empty := thread("b", "high")

		deleted := thread("c", "high", "retracted")
		deleted.Comments[0].Deleted = true

		got := Build(config.Default(), []comments.Thread{resolved, empty, deleted}, "s", "p")
		if len(got.Suggestions) != 0 {
			t.Errorf("got %d suggestions; want 0: %+v", len(got.Suggestions), got.Suggestions)
		}
	})

	t.Run("fills the fields skill-updater requires", func(t *testing.T) {
		bare := comments.Thread{
			ID: "a", Quote: "some text", Status: "open",
			Comments: []comments.Comment{{ID: "c1", Author: "olly", Body: "be specific"}},
		}

		got := Build(config.Default(), []comments.Thread{bare}, "s", "p").Suggestions[0]

		// The id is what a later handoff matches against its archives to know this
		// suggestion has already been applied.
		if got.ID != "a" {
			t.Errorf("id = %q; want the originating thread's", got.ID)
		}
		for _, f := range config.Default().Fields {
			if got.Fields[f.Name] != f.Default {
				t.Errorf("%s = %q; want the default %q", f.Name, got.Fields[f.Name], f.Default)
			}
		}
		if got.ExpectedImpact == "" {
			t.Error("expected_impact is empty; skill-updater requires it")
		}
		if !strings.Contains(got.Suggestion, "some text") {
			t.Errorf("suggestion drops the anchored passage: %q", got.Suggestion)
		}
	})

	t.Run("falls back on unrecognised priority and category", func(t *testing.T) {
		odd := thread("a", "URGENT!!", "do it")
		odd.Fields["category"] = "vibes"

		got := Build(config.Default(), []comments.Thread{odd}, "s", "p").Suggestions[0]

		if got.Fields["priority"] != "medium" || got.Fields["category"] != "instructions" {
			t.Errorf("fields = %+v; want the defaults", got.Fields)
		}
	})

	// The default schema is only a default: a configured one has to replace it whole,
	// including what the suggestions are sorted by.
	t.Run("a custom schema replaces the fields and the ordering", func(t *testing.T) {
		cfg := &config.Config{Fields: []config.Field{
			{Name: "severity", Values: []string{"blocker", "nit"}, Default: "nit"},
			{Name: "area", Values: []string{"prose", "code"}, Default: "prose"},
		}}

		nit := thread("a", "", "second")
		blocker := thread("b", "", "first")
		blocker.Fields = map[string]string{"severity": "blocker", "area": "code"}

		got := Build(cfg, []comments.Thread{nit, blocker}, "s", "p").Suggestions

		if len(got) != 2 || got[0].ID != "b" {
			t.Fatalf("not sorted by severity: %+v", got)
		}
		if got[0].Fields["area"] != "code" || got[1].Fields["severity"] != "nit" {
			t.Errorf("fields = %+v, %+v", got[0].Fields, got[1].Fields)
		}
		if _, ok := got[0].Fields["priority"]; ok {
			t.Error("the default schema leaked into a configured one")
		}
	})

	t.Run("serialises the fields flat alongside the rest", func(t *testing.T) {
		got := Build(config.Default(), []comments.Thread{thread("a", "high", "do it")}, "s", "p")

		body, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		for _, want := range []string{`"priority":"high"`, `"category":"instructions"`, `"cause":"instructions"`, `"id":"a"`, `"improvement_suggestions"`} {
			if !strings.Contains(string(body), want) {
				t.Errorf("payload is missing %s:\n%s", want, body)
			}
		}

		var round Payload
		if err := json.Unmarshal(body, &round); err != nil {
			t.Fatal(err)
		}
		if round.Suggestions[0].Fields["priority"] != "high" || round.Suggestions[0].ID != "a" {
			t.Errorf("round trip lost data: %+v", round.Suggestions[0])
		}
	})

	t.Run("keeps the reviewer's own impact when given", func(t *testing.T) {
		withImpact := thread("a", "high", "add an example")
		withImpact.Impact = "fewer misfires on ambiguous input"

		got := Build(config.Default(), []comments.Thread{withImpact}, "s", "p").Suggestions[0]

		if got.ExpectedImpact != "fewer misfires on ambiguous input" {
			t.Errorf("expected_impact = %q", got.ExpectedImpact)
		}
	})

	t.Run("truncates a very long quote", func(t *testing.T) {
		long := thread("a", "high", "shorten this")
		long.Quote = strings.Repeat("x", quoteLimit*2)

		got := Build(config.Default(), []comments.Thread{long}, "s", "p").Suggestions[0]

		if len(got.Suggestion) > quoteLimit*2 {
			t.Errorf("suggestion is %d chars; a whole code block should not be inlined", len(got.Suggestion))
		}
		if !strings.Contains(got.Suggestion, "…") {
			t.Error("truncation is not signalled")
		}
	})

	t.Run("joins a conversation into one suggestion", func(t *testing.T) {
		conversation := thread("a", "high", "this is vague", "specifically, say which file")

		got := Build(config.Default(), []comments.Thread{conversation}, "s", "p").Suggestions[0]

		for _, want := range []string{"this is vague", "specifically, say which file"} {
			if !strings.Contains(got.Suggestion, want) {
				t.Errorf("suggestion drops %q: %q", want, got.Suggestion)
			}
		}
	})
}
