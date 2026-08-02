package handoff

import (
	"bytes"
	"cmp"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
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
}

// The embedded payload flattens, so the file stays a superset of what the updater reads:
// the prompt and its archive target ride along rather than living only in a browser toast.
type pendingFile struct {
	Payload
	Prompt  string `json:"prompt"`
	Archive string `json:"archive"`
}

// Submit regenerates the pending file from the document rather than appending to it, so a
// thread that was retriaged, replied to, resolved or deleted since the last submit is
// reflected. Only threads already moved to an archive file are excluded, which is what
// stops the updater being handed the same suggestion twice.
//
// It is the only function in this package that touches the filesystem, and both the
// browser's Submit button and the handoff subcommand go through it so the two can never
// disagree.
func Submit(cfg *config.Config, outDir, skillPath string, src []byte) (Result, error) {
	threads, err := comments.Threads(src)
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return Result{}, fmt.Errorf("handoff: create %s: %w", outDir, err)
	}

	file := filepath.Join(outDir, PendingName)
	archived := ArchivedIDs(outDir)
	payload := Build(cfg, threads, skill.Name(src), skillPath)
	payload.Suggestions = slices.DeleteFunc(payload.Suggestions, func(s Suggestion) bool {
		return archived[s.ID]
	})

	if len(payload.Suggestions) == 0 {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			return Result{}, fmt.Errorf("handoff: remove %s: %w", file, err)
		}
		return Result{Payload: payload}, nil
	}

	// Reusing the archive name already recorded keeps a prompt copied earlier valid, and
	// lets an unchanged pending file compare equal.
	archive := readPending(outDir).Archive
	if archive == "" {
		archive = filepath.Join(outDir, fmt.Sprintf("handoff-%s-%s.json",
			cmp.Or(payload.SkillName, "skill"), time.Now().UTC().Format("20060102T150405Z")))
	}
	prompt := Prompt(cfg.Updater, skillPath, file, archive)

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
	return Result{File: file, Prompt: prompt, Changed: changed, Payload: payload}, nil
}

// The mv is the load-bearing half of both prompts: until it runs those threads stay
// pending, and once it has run their ids are archived and never handed off again.
const archiveInstruction = "Once applied, archive it so these suggestions are not handed off again:\nmv %s %s"

// Prompt names the configured skill when there is one. With none configured it spells the
// same work out instead, so the tool is usable without a skill to delegate to.
func Prompt(u config.Updater, skillPath, pending, archive string) string {
	if u.Name != "" {
		return fmt.Sprintf("Use %s with the payload in %s\n\n"+archiveInstruction, u.Name, pending, pending, archive)
	}
	return fmt.Sprintf(
		"Apply the improvement suggestions in %s to the skill at %s.\n\n"+
			"Work through them in the order given. For each one, make the smallest edit to the\n"+
			"SKILL.md that satisfies it, keeping the skill's existing voice and structure — do\n"+
			"not restructure anything a suggestion did not ask about.\n\n"+
			archiveInstruction,
		pending, skillPath, pending, archive)
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
