package handoff

import (
	"strings"
	"testing"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
)

func thread(id, priority string, bodies ...string) comments.Thread {
	t := comments.Thread{ID: id, Quote: "the anchored passage", Status: "open", Priority: priority}
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

		got := Build(threads, "example-skill", "/skills/example/SKILL.md")

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

		got := Build([]comments.Thread{resolved, empty, deleted}, "s", "p")
		if len(got.Suggestions) != 0 {
			t.Errorf("got %d suggestions; want 0: %+v", len(got.Suggestions), got.Suggestions)
		}
	})

	t.Run("fills the fields skill-updater requires", func(t *testing.T) {
		bare := comments.Thread{
			ID: "a", Quote: "some text", Status: "open",
			Comments: []comments.Comment{{ID: "c1", Author: "olly", Body: "be specific"}},
		}

		got := Build([]comments.Thread{bare}, "s", "p").Suggestions[0]

		// The id is what a later handoff matches against its archives to know this
		// suggestion has already been applied.
		if got.ID != "a" {
			t.Errorf("id = %q; want the originating thread's", got.ID)
		}
		if got.Priority != defaultPriority {
			t.Errorf("priority = %q; want %q", got.Priority, defaultPriority)
		}
		if got.Category != defaultCategory {
			t.Errorf("category = %q; want %q", got.Category, defaultCategory)
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
		odd.Category = "vibes"

		got := Build([]comments.Thread{odd}, "s", "p").Suggestions[0]

		if got.Priority != defaultPriority || got.Category != defaultCategory {
			t.Errorf("got priority %q category %q; want defaults", got.Priority, got.Category)
		}
	})

	t.Run("keeps the reviewer's own impact when given", func(t *testing.T) {
		withImpact := thread("a", "high", "add an example")
		withImpact.Impact = "fewer misfires on ambiguous input"

		got := Build([]comments.Thread{withImpact}, "s", "p").Suggestions[0]

		if got.ExpectedImpact != "fewer misfires on ambiguous input" {
			t.Errorf("expected_impact = %q", got.ExpectedImpact)
		}
	})

	t.Run("truncates a very long quote", func(t *testing.T) {
		long := thread("a", "high", "shorten this")
		long.Quote = strings.Repeat("x", quoteLimit*2)

		got := Build([]comments.Thread{long}, "s", "p").Suggestions[0]

		if len(got.Suggestion) > quoteLimit*2 {
			t.Errorf("suggestion is %d chars; a whole code block should not be inlined", len(got.Suggestion))
		}
		if !strings.Contains(got.Suggestion, "…") {
			t.Error("truncation is not signalled")
		}
	})

	t.Run("joins a conversation into one suggestion", func(t *testing.T) {
		conversation := thread("a", "high", "this is vague", "specifically, say which file")

		got := Build([]comments.Thread{conversation}, "s", "p").Suggestions[0]

		for _, want := range []string{"this is vague", "specifically, say which file"} {
			if !strings.Contains(got.Suggestion, want) {
				t.Errorf("suggestion drops %q: %q", want, got.Suggestion)
			}
		}
	})
}

func TestSkillName(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want string
	}{
		{
			name: "frontmatter name",
			src:  "---\nname: skill-updater\ndescription: does things\n---\n\n# Heading\n",
			want: "skill-updater",
		},
		{
			name: "quoted value",
			src:  "---\nname: \"quoted-name\"\n---\n",
			want: "quoted-name",
		},
		{
			name: "no frontmatter",
			src:  "# Just a heading\n",
			want: "",
		},
		{
			name: "name outside the frontmatter block is ignored",
			src:  "---\ndescription: x\n---\n\nname: not-this\n",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SkillName([]byte(tt.src)); got != tt.want {
				t.Errorf("got %q; want %q", got, tt.want)
			}
		})
	}
}
