package handoff

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
	"github.com/O-Marsters-1997/improve-skills/internal/config"
	"github.com/O-Marsters-1997/improve-skills/internal/skill"
)

const PendingName = "pending.json"

type Result struct {
	File    string  `json:"file"`
	Prompt  string  `json:"prompt"`
	Changed bool    `json:"changed"`
	Payload Payload `json:"payload"`
	// Submitted is the ids this call drew from docs, minus any already archived — the
	// ones the caller may now strip from those documents. It excludes ids the payload
	// carries only because an earlier, different submit put them there.
	Submitted []string `json:"submitted"`
}

// The embedded payload flattens, so the file stays a superset of what the updater reads:
// the prompt and its archive target ride along rather than living only in a browser toast.
type pendingFile struct {
	Payload
	Prompt  string `json:"prompt"`
	Archive string `json:"archive"`
}

// A Doc is one reviewed document: where it lives, and the bytes the threads were parsed
// out of. Path is absolute, because it is what the updater is told to look at.
type Doc struct {
	Path string
	Src  []byte
}

// Submit builds suggestions from the reviewed documents and merges them onto whatever is
// already pending, this call's version of a given id winning — so a thread retriaged,
// replied to, resolved or deleted since it was last submitted is reflected, while a
// suggestion from a document not passed this time (the server submits one file at a time)
// is left standing. Only threads already moved to an archive file are excluded, which is
// what stops the updater being handed the same suggestion twice.
//
// skillPath is the skill the payload edits, which is the reviewed document only when no
// --skill was given. The skill's name is read from its own SKILL.md, never from the
// reviewed bytes: a directory review has many documents and only one of them is it.
//
// It is the only function in this package that touches the filesystem, and both the
// browser's Submit button and the handoff subcommand go through it so the two can never
// disagree.
func Submit(cfg *config.Config, outDir, skillPath string, docs []Doc) (Result, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("handoff: create %s: %w", outDir, err)
	}

	skillMD, err := skill.Resolve(skillPath)
	if err != nil {
		return Result{}, err
	}

	payload := Payload{
		SkillName:   skill.NameAt(skillPath),
		SkillPath:   skillPath,
		Mode:        deriveMode(skillMD, docs),
		Suggestions: []Suggestion{},
	}
	var submitted []string
	for _, d := range docs {
		threads, err := comments.Threads(d.Src)
		if err != nil {
			return Result{}, err
		}
		from := Build(cfg, threads)
		for i := range from {
			from[i].File = d.Path
			submitted = append(submitted, from[i].ID)
		}
		payload.Suggestions = append(payload.Suggestions, from...)
	}

	file := filepath.Join(outDir, PendingName)
	archived := ArchivedIDs(outDir)
	submitted = slices.DeleteFunc(submitted, func(id string) bool { return archived[id] })

	// Merge onto whatever is already pending, this run's version winning per id, so
	// submitting one document never erases another's suggestions still awaiting handoff —
	// docs here is one file at a time, everything else pending came from an earlier call.
	previous := readPending(outDir)
	byID := make(map[string]Suggestion, len(previous.Suggestions)+len(payload.Suggestions))
	for _, s := range slices.Concat(previous.Suggestions, payload.Suggestions) {
		if !archived[s.ID] {
			byID[s.ID] = s
		}
	}
	// Rank by id before the priority sort: map iteration is randomised, and sortSuggestions
	// is stable, so without this two runs over the same threads emit the same suggestions in
	// a different order — which would churn the file and make Changed meaningless.
	payload.Suggestions = slices.SortedFunc(maps.Values(byID), func(a, b Suggestion) int {
		return cmp.Compare(a.ID, b.ID)
	})
	sortSuggestions(cfg, payload.Suggestions)

	if len(payload.Suggestions) == 0 {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("handoff: remove %s: %w", file, err)
		}
		return Result{Payload: payload, Submitted: submitted}, nil
	}

	// Reusing the archive name already recorded keeps a prompt copied earlier valid, and
	// lets an unchanged pending file compare equal.
	archive := previous.Archive
	if archive == "" {
		archive = filepath.Join(outDir, fmt.Sprintf("handoff-%s-%s.json",
			cmp.Or(payload.SkillName, "skill"), time.Now().UTC().Format("20060102T150405Z")))
	}
	prompt := Prompt(cfg.Updater, payload.Mode, skillPath, file, archive)

	body, err := json.MarshalIndent(pendingFile{Payload: payload, Prompt: prompt, Archive: archive}, "", "  ")
	if err != nil {
		return Result{}, fmt.Errorf("handoff: encode payload: %w", err)
	}

	changed := true
	if existing, err := os.ReadFile(file); err == nil {
		changed = !bytes.Equal(existing, body)
	}
	if changed {
		if err := os.WriteFile(file, body, 0o644); err != nil {
			return Result{}, fmt.Errorf("handoff: write %s: %w", file, err)
		}
	}
	return Result{File: file, Prompt: prompt, Changed: changed, Payload: payload, Submitted: submitted}, nil
}

// A document counts as instructions only when every reviewed file resolves inside the
// skill. Symlinks are resolved on both sides first: ~/.claude/skills is largely a symlink
// farm into ~/.agents/skills, and without that step every real skill reads as output.
//
// The skill's directory is taken from the resolved SKILL.md rather than resolved as a
// directory, because either half of the pair can be the link — the farm holds directory
// symlinks in some installs and file symlinks in others.
func deriveMode(skillMD string, docs []Doc) string {
	skillDir := filepath.Dir(resolveLinks(skillMD))
	for _, d := range docs {
		rel, err := filepath.Rel(skillDir, resolveLinks(d.Path))
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return ModeOutput
		}
	}
	return ModeInstructions
}

// A path that does not exist yet cannot be resolved, and is no worse off unresolved.
func resolveLinks(path string) string {
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		return resolved
	}
	return path
}

// Prompt names the configured skill when there is one. With none configured it spells the
// same work out instead, so the tool is usable without a skill to delegate to. In output
// mode it adds the inference step, which is the whole difference between the two modes:
// the suggestions describe a document the skill wrote, not the skill.
func Prompt(u config.Updater, mode, skillPath, pending, archive string) string {
	var b strings.Builder
	if u.Name != "" {
		fmt.Fprintf(&b, "Use %s with the payload in %s\n\n", u.Name, pending)
	} else {
		fmt.Fprintf(&b,
			"Apply the improvement suggestions in %s to the skill at %s.\n\n"+
				"Work through them in the order given. For each one, make the smallest edit to the\n"+
				"SKILL.md that satisfies it, keeping the skill's existing voice and structure — do\n"+
				"not restructure anything a suggestion did not ask about.\n\n",
			pending, skillPath)
	}
	if mode == ModeOutput {
		fmt.Fprintf(&b,
			"These suggestions are observations about a document the skill produced, named by\n"+
				"the \"file\" key on each one — not about the skill's own text. For each, infer which\n"+
				"instruction in the skill at %s allowed it, and edit that SKILL.md:\n"+
				"never the reviewed file.\n\n"+
				"Weigh them as one review, not as independent edits. Several observations often\n"+
				"trace back to the same instruction, and one read on its own invites a fix the rest\n"+
				"contradict. Where an observation traces back to no instruction, report it and\n"+
				"leave the SKILL.md alone — an edit invented to cover it is a rule the skill never\n"+
				"needed.\n\n",
			skillPath)
	}
	// The mv is the load-bearing half of every prompt: until it runs those threads stay
	// pending, and once it has run their ids are archived and never handed off again.
	fmt.Fprintf(&b, "Once applied, archive it so these suggestions are not handed off again:\nmv %s %s", pending, archive)
	return b.String()
}

func readPending(outDir string) pendingFile {
	var p pendingFile
	if body, err := os.ReadFile(filepath.Join(outDir, PendingName)); err == nil {
		_ = json.Unmarshal(body, &p)
	}
	return p
}

// ArchivedIDs is every thread already moved into an archive file. One that will not decode
// is skipped rather than fatal — but noisily, since ignoring it silently would let an
// already-applied suggestion come back.
//
// Submit uses it to exclude, and the server to avoid minting an id it would exclude.
func ArchivedIDs(outDir string) map[string]bool {
	ids := map[string]bool{}
	entries, err := filepath.Glob(filepath.Join(outDir, "*.json"))
	if err != nil {
		log.Printf("handoff: cannot list %s: %v", outDir, err)
		return ids
	}
	for _, entry := range entries {
		if filepath.Base(entry) == PendingName {
			continue
		}
		var payload Payload
		body, err := os.ReadFile(entry)
		if err == nil {
			err = json.Unmarshal(body, &payload)
		}
		if err != nil {
			log.Printf("handoff: skipping %s: %v", entry, err)
			continue
		}
		for _, suggestion := range payload.Suggestions {
			ids[suggestion.ID] = true
		}
	}
	return ids
}
