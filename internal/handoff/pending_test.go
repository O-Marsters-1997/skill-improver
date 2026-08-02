package handoff

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func writeSkill(t *testing.T, dir string) string {
	t.Helper()

	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, skill.FileName), []byte("---\nname: ideate\n---\n\n# Ideate\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func submit(t *testing.T, outDir, skillPath string, docs ...Doc) Result {
	t.Helper()

	got, err := Submit(config.Default(), outDir, skillPath, docs)
	if err != nil {
		t.Fatal(err)
	}
	return got
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

// n open threads all left at the default priority, so nothing but the id can tie-break
// them once sortSuggestions has run.
func writeTiedDoc(t *testing.T, path string, n int) Doc {
	t.Helper()

	var body, block strings.Builder
	body.WriteString("# Report\n\n")
	block.WriteString("<!--mc:threads:begin-->\n")
	for i := range n {
		id := fmt.Sprintf("id%02d", i)
		fmt.Fprintf(&body, "<!--mc:a:%s-->claim %d<!--mc:/a:%s-->\n\n", id, i, id)
		fmt.Fprintf(&block, "<!--mc:t {\"id\":%q,\"quote\":\"claim %d\",\"status\":\"open\","+
			"\"comments\":[{\"id\":\"c1\",\"author\":\"olly\",\"ts\":\"2026-08-02T00:00:00Z\",\"body\":\"say why\"}]}-->\n", id, i)
	}
	block.WriteString("<!--mc:threads:end-->\n")

	return writeDoc(t, path, body.String()+block.String())
}

// The merge collects out of a map, and Go randomises map iteration, so a stable sort on
// its own leaves same-priority suggestions in a different order on every run.
func TestSubmitIsDeterministic(t *testing.T) {
	root := t.TempDir()
	skillDir := writeSkill(t, filepath.Join(root, "ideate"))
	doc := writeTiedDoc(t, filepath.Join(root, "reports", "2026-08-02-ideate.md"), 8)
	outDir := filepath.Join(root, "out")

	first := submit(t, outDir, skillDir, doc)
	want, err := os.ReadFile(first.File)
	if err != nil {
		t.Fatal(err)
	}

	for i := range 5 {
		got := submit(t, outDir, skillDir, doc)
		if got.Changed {
			t.Errorf("resubmit %d reports the pending file changed; the threads did not", i)
		}
		body, err := os.ReadFile(got.File)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(body, want) {
			t.Fatalf("resubmit %d reordered the pending file:\ngot:\n%s\nwant:\n%s", i, body, want)
		}
	}
}

func TestSubmitNamesTheSkillNotTheDocument(t *testing.T) {
	root := t.TempDir()
	skillDir := writeSkill(t, filepath.Join(root, "skills", "ideate"))
	doc := writeDoc(t, filepath.Join(root, "reports", "2026-08-02-ideate.md"))

	got := submit(t, filepath.Join(root, "out"), skillDir, doc)

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
		skillDir := writeSkill(t, filepath.Join(root, "ideate"))
		doc := writeDoc(t, filepath.Join(skillDir, "references", "notes.md"))

		got := submit(t, filepath.Join(root, "out"), skillDir, doc)
		if got.Payload.Mode != ModeInstructions {
			t.Errorf("mode = %q; want %q", got.Payload.Mode, ModeInstructions)
		}
	})

	// ~/.claude/skills is largely a symlink farm into ~/.agents/skills. Without
	// resolving both sides every real skill computes as output.
	t.Run("a skill reached through a symlink is instructions", func(t *testing.T) {
		root := t.TempDir()
		actual := writeSkill(t, filepath.Join(root, "agents", "ideate"))
		doc := writeDoc(t, filepath.Join(actual, skill.FileName))

		link := filepath.Join(root, "claude", "ideate")
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(actual, link); err != nil {
			t.Fatal(err)
		}

		got := submit(t, filepath.Join(root, "out"), link, doc)
		if got.Payload.Mode != ModeInstructions {
			t.Errorf("mode = %q; want %q", got.Payload.Mode, ModeInstructions)
		}
		if got.Payload.SkillPath != link {
			t.Errorf("skill_path = %q; want the path the reviewer gave, %q", got.Payload.SkillPath, link)
		}
	})

	// The other half of the farm: some installs link the directory, some link the
	// SKILL.md into a directory that is otherwise real.
	t.Run("a SKILL.md reached through a symlink is instructions", func(t *testing.T) {
		root := t.TempDir()
		actual := writeSkill(t, filepath.Join(root, "agents", "ideate"))

		linked := filepath.Join(root, "claude", "ideate")
		if err := os.MkdirAll(linked, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(actual, skill.FileName), filepath.Join(linked, skill.FileName)); err != nil {
			t.Fatal(err)
		}
		doc := Doc{Path: filepath.Join(actual, skill.FileName), Src: []byte(reviewed)}

		got := submit(t, filepath.Join(root, "out"), linked, doc)
		if got.Payload.Mode != ModeInstructions {
			t.Errorf("mode = %q; want %q", got.Payload.Mode, ModeInstructions)
		}
	})

	// The default: no --skill, so the target is the skill and nothing changes.
	t.Run("reviewing a SKILL.md with no separate skill is instructions", func(t *testing.T) {
		root := t.TempDir()
		skillDir := filepath.Join(root, "ideate")
		doc := writeDoc(t, filepath.Join(skillDir, skill.FileName), "---\nname: ideate\n---\n\n"+reviewed)

		got := submit(t, filepath.Join(root, "out"), doc.Path, doc)
		if got.Payload.Mode != ModeInstructions || got.Payload.SkillName != "ideate" {
			t.Errorf("payload = %+v", got.Payload)
		}
	})
}

// Submit is called once per file by the server now, so a later call for a different
// document must add to what an earlier call left pending, not replace it.
func TestSubmitAccumulatesAcrossCalls(t *testing.T) {
	root := t.TempDir()
	skillDir := writeSkill(t, filepath.Join(root, "ideate"))
	outDir := filepath.Join(root, "out")

	docA := writeDoc(t, filepath.Join(root, "a.md"))
	docB := writeDoc(t, filepath.Join(root, "b.md"), strings.ReplaceAll(reviewed, "aaa", "bbb"))

	first := submit(t, outDir, skillDir, docA)
	if len(first.Payload.Suggestions) != 1 || first.Payload.Suggestions[0].File != docA.Path {
		t.Fatalf("first submit = %+v", first.Payload.Suggestions)
	}

	second := submit(t, outDir, skillDir, docB)
	if len(second.Payload.Suggestions) != 2 {
		t.Fatalf("got %d suggestions; want doc A's to survive alongside doc B's", len(second.Payload.Suggestions))
	}
	if got := second.Submitted; len(got) != 1 || got[0] != "bbb" {
		t.Errorf("submitted = %v; want only bbb, the ids this call drew from docB", got)
	}
}

// A retriage between two submits of the same document has to win over the version
// already sitting in pending.json — the merge keys on id, this call's copy replacing it.
func TestSubmitOfTheSameDocReplacesItsOwnEntry(t *testing.T) {
	root := t.TempDir()
	skillDir := writeSkill(t, filepath.Join(root, "ideate"))
	outDir := filepath.Join(root, "out")

	retriaged := strings.Replace(reviewed, `"status":"open"`, `"status":"open","priority":"high"`, 1)
	docV1 := writeDoc(t, filepath.Join(root, "a.md"))
	docV2 := writeDoc(t, filepath.Join(root, "a.md"), retriaged)

	submit(t, outDir, skillDir, docV1)
	got := submit(t, outDir, skillDir, docV2)
	if len(got.Payload.Suggestions) != 1 {
		t.Fatalf("got %d suggestions; want the resubmit to replace, not duplicate", len(got.Payload.Suggestions))
	}
	if got.Payload.Suggestions[0].Fields["priority"] != "high" {
		t.Errorf("suggestion = %+v; want the retriaged priority", got.Payload.Suggestions[0])
	}
}

// Submitted is what the caller may now strip from the document — an id already archived
// must never come back for a second round, so it must not appear there either.
func TestSubmitExcludesArchivedFromSubmitted(t *testing.T) {
	root := t.TempDir()
	skillDir := writeSkill(t, filepath.Join(root, "ideate"))
	outDir := filepath.Join(root, "out")
	doc := writeDoc(t, filepath.Join(root, "a.md"))

	archive, err := json.Marshal(Payload{Suggestions: []Suggestion{{ID: "aaa"}}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outDir, "handoff-old.json"), archive, 0o644); err != nil {
		t.Fatal(err)
	}

	got := submit(t, outDir, skillDir, doc)
	if len(got.Submitted) != 0 {
		t.Errorf("submitted = %v; want the archived id excluded", got.Submitted)
	}
	if len(got.Payload.Suggestions) != 0 {
		t.Errorf("payload = %+v; want the archived thread left out entirely", got.Payload.Suggestions)
	}
}

func TestPromptTellsTheUpdaterWhatItIsLookingAt(t *testing.T) {
	root := t.TempDir()
	skillDir := writeSkill(t, filepath.Join(root, "ideate"))
	doc := writeDoc(t, filepath.Join(root, "report.md"))

	got := submit(t, filepath.Join(root, "out"), skillDir, doc)
	// The #3 spike found the inference itself robust but the edit calibration fragile:
	// a lone undiagnosed observation drew an invented rule that contradicted an earlier
	// run. Both guards against that have to survive in the prompt.
	want := []string{
		"produced", skillDir, "never the reviewed file",
		"as one review, not as independent edits",
		"traces back to no instruction, report it",
	}
	for _, w := range want {
		if !strings.Contains(got.Prompt, w) {
			t.Errorf("prompt does not mention %q:\n%s", w, got.Prompt)
		}
	}
}
