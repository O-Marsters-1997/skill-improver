package handoff

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/O-Marsters-1997/improve-skills/internal/config"
	"github.com/O-Marsters-1997/improve-skills/internal/skill"
)

// One open thread, written the way the server writes it. Deliberately without
// frontmatter: the skill's name must not come from the reviewed bytes.
const reviewed = `# Report

<!--mc:a:aaa-->A claim that is wrong.<!--mc:/a:aaa-->

<!--mc:threads:begin-->
<!--mc:t {"id":"aaa","quote":"A claim that is wrong.","status":"open","comments":[{"id":"c1","author":"olly","ts":"2026-08-02T00:00:00Z","body":"say why"}]}-->
<!--mc:threads:end-->
`

func writeSkill(t *testing.T, dir string) {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, skill.FileName), []byte("---\nname: ideate\n---\n\n# Ideate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeDoc(t *testing.T, path string, src ...string) Doc {
	t.Helper()

	body := []byte(reviewed)
	if len(src) == 1 {
		body = []byte(src[0])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, body, 0o644); err != nil {
		t.Fatal(err)
	}
	return Doc{Path: path, Src: body}
}

func TestSubmitNamesTheSkillNotTheDocument(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "ideate")
	writeSkill(t, skillDir)
	doc := writeDoc(t, filepath.Join(root, "reports", "2026-08-02-ideate.md"))

	got, err := Submit(config.Default(), filepath.Join(root, "out"), skillDir, []Doc{doc})
	if err != nil {
		t.Fatal(err)
	}

	if got.Payload.Mode != ModeOutput {
		t.Errorf("mode = %q; want %q", got.Payload.Mode, ModeOutput)
	}
	// The name comes from the skill's own frontmatter, never from the reviewed bytes.
	if got.Payload.SkillName != "ideate" || got.Payload.SkillPath != skillDir {
		t.Errorf("payload header = %+v", got.Payload)
	}
	if len(got.Payload.Suggestions) != 1 {
		t.Fatalf("got %d suggestions; want 1", len(got.Payload.Suggestions))
	}
	if file := got.Payload.Suggestions[0].File; file != doc.Path {
		t.Errorf("file = %q; want the reviewed document %q", file, doc.Path)
	}
}

func TestSubmitModeIsDerivedFromThePaths(t *testing.T) {
	t.Run("a file inside the skill is instructions", func(t *testing.T) {
		root := t.TempDir()
		skillDir := filepath.Join(root, "ideate")
		writeSkill(t, skillDir)
		doc := writeDoc(t, filepath.Join(skillDir, "references", "notes.md"))

		got, err := Submit(config.Default(), filepath.Join(root, "out"), skillDir, []Doc{doc})
		if err != nil {
			t.Fatal(err)
		}
		if got.Payload.Mode != ModeInstructions {
			t.Errorf("mode = %q; want %q", got.Payload.Mode, ModeInstructions)
		}
	})

	// ~/.claude/skills is largely a symlink farm into ~/.agents/skills. Without
	// resolving both sides every real skill computes as output.
	t.Run("a skill reached through a symlink is instructions", func(t *testing.T) {
		root := t.TempDir()
		actual := filepath.Join(root, "agents", "ideate")
		writeSkill(t, actual)
		doc := writeDoc(t, filepath.Join(actual, skill.FileName))

		link := filepath.Join(root, "claude", "ideate")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(actual, link); err != nil {
			t.Fatal(err)
		}

		got, err := Submit(config.Default(), filepath.Join(root, "out"), link, []Doc{doc})
		if err != nil {
			t.Fatal(err)
		}
		if got.Payload.Mode != ModeInstructions {
			t.Errorf("mode = %q; want %q", got.Payload.Mode, ModeInstructions)
		}
		if got.Payload.SkillPath != link {
			t.Errorf("skill_path = %q; want the path the reviewer gave, %q", got.Payload.SkillPath, link)
		}
	})

	// The default: no --skill, so the target is the skill and nothing changes.
	t.Run("reviewing a SKILL.md with no separate skill is instructions", func(t *testing.T) {
		root := t.TempDir()
		skillDir := filepath.Join(root, "ideate")
		doc := writeDoc(t, filepath.Join(skillDir, skill.FileName), "---\nname: ideate\n---\n\n"+reviewed)

		got, err := Submit(config.Default(), filepath.Join(root, "out"), doc.Path, []Doc{doc})
		if err != nil {
			t.Fatal(err)
		}
		if got.Payload.Mode != ModeInstructions || got.Payload.SkillName != "ideate" {
			t.Errorf("payload = %+v", got.Payload)
		}
	})
}

func TestPromptTellsTheUpdaterWhatItIsLookingAt(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "ideate")
	writeSkill(t, skillDir)
	doc := writeDoc(t, filepath.Join(root, "report.md"))

	got, err := Submit(config.Default(), filepath.Join(root, "out"), skillDir, []Doc{doc})
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"produced", skillDir, "never the reviewed file"} {
		if !strings.Contains(got.Prompt, want) {
			t.Errorf("prompt does not mention %q:\n%s", want, got.Prompt)
		}
	}
}
