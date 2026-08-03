package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
	"github.com/O-Marsters-1997/improve-skills/internal/config"
	"github.com/O-Marsters-1997/improve-skills/internal/handoff"
)

const fixture = `---
name: example-skill
description: a fixture
---

# Example Skill

Comments autosave to disk; never push to a terminal.
`

// The fixture is copied into a temp directory so a test can mutate it freely.
func newFileServer(t *testing.T, cfg *config.Config, name, body string) (*Server, string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	s, err := New(cfg, path, "", filepath.Join(dir, "out"), "olly")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, path
}

func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	return newFileServer(t, config.Default(), "SKILL.md", fixture)
}

func post(t *testing.T, s *Server, path string, body any) *httptest.ResponseRecorder {
	t.Helper()

	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("encode request: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(encoded))
	w := httptest.NewRecorder()
	s.ServeHTTP(w, req)
	return w
}

func getFile(t *testing.T, s *Server, rel string) doc {
	t.Helper()

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/doc?file="+rel, nil))
	return decodeDoc(t, w)
}

// An empty rel means the first file in the set, the way the page sends it for a
// single-file review.
func getDoc(t *testing.T, s *Server) doc {
	t.Helper()
	return getFile(t, s, "")
}

func decodeDoc(t *testing.T, w *httptest.ResponseRecorder) doc {
	t.Helper()

	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var d doc
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode doc: %v", err)
	}
	return d
}

func offsetOf(t *testing.T, src, passage string) (int, int) {
	t.Helper()

	i := strings.Index(src, passage)
	if i < 0 {
		t.Fatalf("passage %q not in source", passage)
	}
	return i, i + len(passage)
}

func TestDoc(t *testing.T) {
	s, _ := newTestServer(t)

	d := getDoc(t, s)

	if d.Name != "example-skill" {
		t.Errorf("name = %q; want example-skill", d.Name)
	}
	if d.Rev == "" {
		t.Error("rev is empty; the conflict check depends on it")
	}
	if !strings.Contains(d.HTML, `data-o="`) {
		t.Errorf("rendered HTML carries no offsets:\n%s", d.HTML)
	}
	if len(d.Threads) != 0 {
		t.Errorf("got %d threads; want 0", len(d.Threads))
	}
}

// The page builds its controls from what this serves, so these three are the whole
// contract between the config and the browser.
func TestDocServesTheSchema(t *testing.T) {
	cfg := &config.Config{
		Fields:  []config.Field{{Name: "severity", Label: "How bad", Values: []string{"blocker", "nit"}, Default: "nit"}},
		Updater: config.Updater{Name: "my-updater"},
	}
	s, path := newFileServer(t, cfg, "SKILL.md", fixture)

	d := getDoc(t, s)

	if len(d.Fields) != 1 || d.Fields[0].Name != "severity" || d.Fields[0].Label != "How bad" {
		t.Errorf("fields = %+v; want the configured one", d.Fields)
	}
	if d.Updater != "my-updater" {
		t.Errorf("updater = %q; the Submit button label comes from this", d.Updater)
	}

	start, end := offsetOf(t, fixture, "never push")
	post(t, s, "/api/anchor", anchorRequest{Rev: d.Rev, Start: start, End: end, Quote: "never push", Body: "x"})
	id := getDoc(t, s).Threads[0].ID

	t.Run("a configured field is written flat onto the thread", func(t *testing.T) {
		got := decodeDoc(t, post(t, s, "/api/thread", threadRequest{
			Rev: getDoc(t, s).Rev, ID: id, Fields: map[string]string{"severity": "blocker"},
		}))
		if got.Threads[0].Fields["severity"] != "blocker" {
			t.Errorf("fields = %+v", got.Threads[0].Fields)
		}
		if !strings.Contains(string(mustRead(t, path)), `"severity":"blocker"`) {
			t.Error("the field did not reach the file")
		}
	})

	// A page left open across a config change must not be able to write a key the
	// payload will never carry.
	t.Run("a field that is not configured is ignored", func(t *testing.T) {
		got := decodeDoc(t, post(t, s, "/api/thread", threadRequest{
			Rev: getDoc(t, s).Rev, ID: id, Fields: map[string]string{"priority": "high"},
		}))
		if _, ok := got.Threads[0].Fields["priority"]; ok {
			t.Errorf("accepted an unconfigured field: %+v", got.Threads[0].Fields)
		}
	})
}

func TestAnchorWritesToDisk(t *testing.T) {
	s, path := newTestServer(t)
	start, end := offsetOf(t, fixture, "never push")

	d := decodeDoc(t, post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: "never push",
		Body: "say why this matters",
	}))

	if len(d.Threads) != 1 {
		t.Fatalf("got %d threads; want 1", len(d.Threads))
	}
	thread := d.Threads[0]
	if thread.Quote != "never push" || len(thread.Fields) != 0 {
		t.Errorf("thread = %+v", thread)
	}
	if len(thread.Comments) != 1 || thread.Comments[0].Author != "olly" || thread.Comments[0].TS == "" {
		t.Errorf("comment = %+v", thread.Comments)
	}

	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	want := "<!--mc:a:" + thread.ID + "-->never push<!--mc:/a:" + thread.ID + "-->"
	if !strings.Contains(string(onDisk), want) {
		t.Errorf("marker pair missing from the file:\n%s", onDisk)
	}
	if !strings.Contains(d.HTML, `class="mc" data-id="`+thread.ID+`"`) {
		t.Errorf("highlight missing from the re-render:\n%s", d.HTML)
	}
}

// Selecting from the very start of a paragraph puts the opening marker in column 0, which
// CommonMark reads as an HTML block — and the renderer used to drop the whole line with it,
// so commenting made the paragraph disappear.
func TestAnchorAtTheStartOfAParagraphKeepsItRendered(t *testing.T) {
	s, _ := newTestServer(t)
	start, end := offsetOf(t, fixture, "Comments autosave")

	d := decodeDoc(t, post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: "Comments autosave", Body: "x",
	}))

	if !strings.Contains(d.HTML, "never push to a terminal") {
		t.Errorf("the commented paragraph vanished from the re-render:\n%s", d.HTML)
	}
	if !strings.Contains(d.HTML, `class="mc" data-id="`+d.Threads[0].ID+`"`) {
		t.Errorf("highlight missing from the re-render:\n%s", d.HTML)
	}
}

func TestAnchorRejectsStaleRevision(t *testing.T) {
	s, path := newTestServer(t)
	start, end := offsetOf(t, fixture, "never push")
	stale := getDoc(t, s).Rev

	if err := os.WriteFile(path, []byte(fixture+"\nEdited elsewhere.\n"), 0o644); err != nil {
		t.Fatalf("simulate an edit in the editor: %v", err)
	}

	w := post(t, s, "/api/anchor", anchorRequest{Rev: stale, Start: start, End: end, Quote: "never push", Body: "x"})

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d; want 409 so the page reloads instead of clobbering", w.Code)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if !strings.Contains(string(after), "Edited elsewhere.") {
		t.Error("the external edit was overwritten")
	}
}

func TestAnchorRejectsImpossibleRange(t *testing.T) {
	s, _ := newTestServer(t)

	w := post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: 10, End: 5, Quote: "x", Body: "y",
	})

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d; want 422", w.Code)
	}
}

func TestThreadLifecycle(t *testing.T) {
	s, _ := newTestServer(t)
	start, end := offsetOf(t, fixture, "never push")
	created := decodeDoc(t, post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: "never push", Body: "first",
	}))
	id := created.Threads[0].ID

	t.Run("reply", func(t *testing.T) {
		d := decodeDoc(t, post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: id, Body: "second"}))
		got := d.Threads[0].Comments
		if len(got) != 2 || got[1].Body != "second" || got[1].ID != "c2" || got[1].Parent != "c1" {
			t.Errorf("comments = %+v", got)
		}
	})

	t.Run("resolve", func(t *testing.T) {
		d := decodeDoc(t, post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: id, Status: "resolved"}))
		if d.Threads[0].Status != "resolved" {
			t.Errorf("status = %q", d.Threads[0].Status)
		}
		if len(d.Threads[0].Comments) != 2 {
			t.Error("resolving dropped the conversation")
		}
	})

	t.Run("change category without touching the rest", func(t *testing.T) {
		d := decodeDoc(t, post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: id, Fields: map[string]string{"category": "examples"}}))
		if d.Threads[0].Fields["category"] != "examples" || d.Threads[0].Status != "resolved" {
			t.Errorf("thread = %+v", d.Threads[0])
		}
	})

	t.Run("unknown thread", func(t *testing.T) {
		w := post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: "zzz", Body: "x"})
		if w.Code != http.StatusNotFound {
			t.Errorf("status = %d; want 404", w.Code)
		}
	})

	t.Run("delete restores the prose", func(t *testing.T) {
		d := decodeDoc(t, post(t, s, "/api/thread/delete", threadRequest{Rev: getDoc(t, s).Rev, ID: id}))
		if len(d.Threads) != 0 {
			t.Errorf("got %d threads; want 0", len(d.Threads))
		}
		if strings.Contains(d.HTML, "mc:a:") {
			t.Errorf("markers left behind:\n%s", d.HTML)
		}
	})
}

// An empty rel means the first file in the set, the way the page sends it for a
// single-file review.
func commentOn(t *testing.T, s *Server, rel, passage, body string) {
	t.Helper()

	_, path, err := s.at(rel)
	if err != nil {
		t.Fatalf("%q is not in the review set: %v", rel, err)
	}
	start, end := offsetOf(t, string(mustRead(t, path)), passage)
	w := post(t, s, "/api/anchor", anchorRequest{
		File: rel, Rev: getFile(t, s, rel).Rev, Start: start, End: end, Quote: passage, Body: body,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("anchor %q in %s = %d: %s", passage, path, w.Code, w.Body)
	}
}

func comment(t *testing.T, s *Server, passage, body string) {
	t.Helper()
	commentOn(t, s, "", passage, body)
}

func submit(t *testing.T, s *Server) handoff.Result {
	t.Helper()
	return submitFile(t, s, "")
}

func submitFile(t *testing.T, s *Server, rel string) handoff.Result {
	t.Helper()

	w := post(t, s, "/api/handoff", fileRequest{File: rel, Rev: getFile(t, s, rel).Rev})
	if w.Code != http.StatusOK {
		t.Fatalf("handoff %q = %d: %s", rel, w.Code, w.Body)
	}
	var got handoff.Result
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode handoff: %v", err)
	}
	return got
}

// Mirrors the unexported shape handoff writes, so a change to it is caught here.
type pendingFile struct {
	handoff.Payload
	Prompt  string `json:"prompt"`
	Archive string `json:"archive"`
}

func readPendingFile(t *testing.T, s *Server) pendingFile {
	t.Helper()

	var p pendingFile
	if err := json.Unmarshal(mustRead(t, filepath.Join(s.outDir, handoff.PendingName)), &p); err != nil {
		t.Fatalf("decode pending: %v", err)
	}
	return p
}

func mustRead(t *testing.T, path string) []byte {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return body
}

// Stands in for the mv the reviewer runs once skill-updater has applied the payload.
func archive(t *testing.T, s *Server) {
	t.Helper()

	if err := os.Rename(filepath.Join(s.outDir, handoff.PendingName), readPendingFile(t, s).Archive); err != nil {
		t.Fatalf("archive: %v", err)
	}
}

func TestHandoffWritesPending(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")

	got := submit(t, s)
	if len(got.Payload.Suggestions) != 1 || !got.Changed {
		t.Fatalf("got %d suggestions, changed=%v", len(got.Payload.Suggestions), got.Changed)
	}
	// A thread never triaged in the sidebar still has to reach skill-updater with a
	// priority and category it accepts.
	if s := got.Payload.Suggestions[0]; s.ID == "" || s.Fields["priority"] != "medium" || s.Fields["category"] != "instructions" || s.ExpectedImpact == "" {
		t.Errorf("untriaged suggestion = %+v", s)
	}
	if got.Payload.SkillName != "example-skill" || got.Payload.SkillPath != s.Path() {
		t.Errorf("payload header = %+v", got.Payload)
	}
	if got.File != filepath.Join(s.outDir, handoff.PendingName) {
		t.Errorf("wrote %q; want the flat pending file under %q", got.File, s.outDir)
	}

	onDisk := readPendingFile(t, s)
	if len(onDisk.Suggestions) != 1 || onDisk.SkillName != "example-skill" {
		t.Errorf("pending is not the skill-updater shape: %+v", onDisk)
	}
	// The prompt has to outlive the browser toast that used to be its only home, and
	// has to carry the archive step, or the next submit repeats itself.
	if onDisk.Prompt != got.Prompt || !strings.Contains(got.Prompt, "mv "+got.File) {
		t.Errorf("prompt on disk = %q; response = %q", onDisk.Prompt, got.Prompt)
	}
	if filepath.Dir(onDisk.Archive) != s.outDir || !strings.HasPrefix(filepath.Base(onDisk.Archive), "handoff-example-skill-") {
		t.Errorf("archive target = %q", onDisk.Archive)
	}
}

func TestHandoffSkipsArchived(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	first := submit(t, s)
	archive(t, s)

	empty := submit(t, s)
	if len(empty.Payload.Suggestions) != 0 {
		t.Fatalf("archived thread came back: %+v", empty.Payload.Suggestions)
	}
	if _, err := os.Stat(filepath.Join(s.outDir, handoff.PendingName)); !os.IsNotExist(err) {
		t.Errorf("pending file survived with nothing pending: %v", err)
	}

	comment(t, s, "Comments autosave", "say where they autosave to")
	second := submit(t, s)
	if len(second.Payload.Suggestions) != 1 {
		t.Fatalf("got %d suggestions; want only the new one", len(second.Payload.Suggestions))
	}
	if second.Payload.Suggestions[0].ID == first.Payload.Suggestions[0].ID {
		t.Errorf("the archived thread was handed off a second time")
	}
}

// Submit now strips a thread from the document the moment it is handed off, so a retriage
// only has anything left to change if it happens before that submit — this pins that a
// field set beforehand is the one that reaches pending.
func TestHandoffReflectsRetriage(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	id := getDoc(t, s).Threads[0].ID

	post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: id, Fields: map[string]string{"priority": "high"}})

	got := submit(t, s)
	if got.Payload.Suggestions[0].Fields["priority"] != "high" || !got.Changed {
		t.Errorf("retriage did not reach pending: %+v changed=%v", got.Payload.Suggestions[0], got.Changed)
	}
}

func TestHandoffSkipsUnchangedWrite(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	submit(t, s)

	before, err := os.Stat(filepath.Join(s.outDir, handoff.PendingName))
	if err != nil {
		t.Fatalf("stat pending: %v", err)
	}

	got := submit(t, s)
	if got.Changed {
		t.Errorf("second submit reported a change with nothing edited")
	}
	after, err := os.Stat(filepath.Join(s.outDir, handoff.PendingName))
	if err != nil {
		t.Fatalf("stat pending: %v", err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("pending was rewritten with nothing new")
	}
}

// A stray file in the output directory must not take the handoff down with it.
// The whole point of the lifecycle fix: a handed-off thread cannot come back for a second
// round because it is gone from the document, but a thread that was never handed off —
// resolved, here — was never in the payload and has no business disappearing either.
func TestHandoffStripsOnlyWhatItHandsOff(t *testing.T) {
	s, path := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	open := getDoc(t, s).Threads[0].ID

	start, end := offsetOf(t, fixture, "Comments autosave")
	resolved := decodeDoc(t, post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: "Comments autosave", Body: "note",
	}))
	post(t, s, "/api/thread", threadRequest{Rev: resolved.Rev, ID: resolved.Threads[1].ID, Status: "resolved"})

	got := submit(t, s)
	if len(got.Payload.Suggestions) != 1 || got.Payload.Suggestions[0].ID != open {
		t.Fatalf("payload = %+v; want only the open thread", got.Payload.Suggestions)
	}

	after := getDoc(t, s)
	if len(after.Threads) != 1 || after.Threads[0].Status != "resolved" {
		t.Errorf("threads after submit = %+v; want only the resolved one left", after.Threads)
	}
	if strings.Contains(string(mustRead(t, path)), open) {
		t.Errorf("the handed-off id is still in the file:\n%s", mustRead(t, path))
	}
}

// Submit strips only the file it was called on; the rest of the review set does not lose
// so much as a byte, and a later submit of a different file still finds its thread there.
func TestHandoffLeavesOtherFilesUntouched(t *testing.T) {
	s, dir := newDirServer(t)
	commentOn(t, s, "SKILL.md", "never push", "explain the failure mode")
	commentOn(t, s, "references/api.md", "The reference body", "say what it returns")

	before := mustRead(t, filepath.Join(dir, "references", "api.md"))

	got := submitFile(t, s, "SKILL.md")
	if len(got.Payload.Suggestions) != 1 || got.Payload.Suggestions[0].File != filepath.Join(dir, "SKILL.md") {
		t.Fatalf("payload = %+v; want only SKILL.md's thread", got.Payload.Suggestions)
	}

	after := mustRead(t, filepath.Join(dir, "references", "api.md"))
	if string(before) != string(after) {
		t.Errorf("references/api.md changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if got := getFile(t, s, "references/api.md"); len(got.Threads) != 1 {
		t.Errorf("references/api.md threads = %+v; want its comment untouched", got.Threads)
	}
}

// The rev check on /api/handoff mirrors mutate's: a document changed since the browser
// last loaded it must not be silently submitted out from under the editor.
func TestHandoffRejectsStaleRevision(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")

	w := post(t, s, "/api/handoff", fileRequest{Rev: "stale"})
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d; want 409", w.Code)
	}
}

func TestFileClearRemovesOnlyThatFilesComments(t *testing.T) {
	s, dir := newDirServer(t)
	commentOn(t, s, "SKILL.md", "never push", "explain the failure mode")
	commentOn(t, s, "references/api.md", "The reference body", "say what it returns")

	cleared := decodeDoc(t, post(t, s, "/api/file/clear", fileRequest{Rev: getDoc(t, s).Rev}))
	if len(cleared.Threads) != 0 {
		t.Errorf("threads after clear = %+v; want none", cleared.Threads)
	}
	if strings.Contains(string(mustRead(t, filepath.Join(dir, "SKILL.md"))), "mc:") {
		t.Errorf("markers left behind in SKILL.md:\n%s", mustRead(t, filepath.Join(dir, "SKILL.md")))
	}

	other := getFile(t, s, "references/api.md")
	if len(other.Threads) != 1 {
		t.Errorf("references/api.md threads = %+v; want its comment untouched", other.Threads)
	}
}

func TestHandoffToleratesCorruptArchive(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	if err := os.MkdirAll(s.outDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(s.outDir, "handoff-broken.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write corrupt archive: %v", err)
	}

	if got := submit(t, s); len(got.Payload.Suggestions) != 1 {
		t.Errorf("got %d suggestions; want the handoff to proceed", len(got.Payload.Suggestions))
	}
}

// Hands out the scripted ids in order, then real ones. A collision is too rare to wait
// for and too quiet to notice, so it has to be forced.
func scriptedIDs(ids ...string) func() string {
	return func() string {
		if len(ids) == 0 {
			return comments.NewID()
		}
		id := ids[0]
		ids = ids[1:]
		return id
	}
}

// Submit deletes any suggestion whose id is archived, so a thread that draws an archived
// id is never handed off and says nothing about it.
func TestAnchorAvoidsArchivedIDs(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	taken := submit(t, s).Payload.Suggestions[0].ID
	archive(t, s)

	// Only the archive holds the id now, so nothing in the document can catch it.
	post(t, s, "/api/thread/delete", threadRequest{Rev: getDoc(t, s).Rev, ID: taken})

	s.newID = scriptedIDs(taken)
	comment(t, s, "Comments autosave", "say where they autosave to")

	got := submit(t, s)
	if len(got.Payload.Suggestions) != 1 {
		t.Fatalf("the new thread never reached the payload: %+v", got.Payload.Suggestions)
	}
	if got.Payload.Suggestions[0].ID == taken {
		t.Errorf("reused the archived id %q", taken)
	}
}

func TestAnchorAvoidsIDsInTheDocument(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	taken := getDoc(t, s).Threads[0].ID

	s.newID = scriptedIDs(taken)
	comment(t, s, "Comments autosave", "say where they autosave to")

	threads := getDoc(t, s).Threads
	if len(threads) != 2 {
		t.Fatalf("got %d threads; want both", len(threads))
	}
	if threads[0].ID == threads[1].ID {
		t.Errorf("both threads got id %q", taken)
	}
}

// An id source that can never produce a free id has to fail the request rather than spin.
func TestAnchorGivesUpOnAnIDSourceThatOnlyCollides(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	taken := getDoc(t, s).Threads[0].ID

	s.newID = func() string { return taken }
	start, end := offsetOf(t, string(mustRead(t, s.Path())), "Comments autosave")
	w := post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: "Comments autosave", Body: "x",
	})

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500 rather than a silently dropped thread", w.Code)
	}
}

// A skill directory as the target: a SKILL.md, a reference, and a pile of things
// discovery has to leave out.
func newDirServer(t *testing.T) (*Server, string) {
	t.Helper()

	dir := t.TempDir()
	write := func(rel, body string) {
		path := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("SKILL.md", fixture)
	write("references/api.md", "# API\n\nThe reference body.\n")
	write("references/notes.md", "# Notes\n\nSomething else entirely.\n")
	write("report.html", "<p>rendered later</p>")
	write("README.txt", "not reviewable")
	write(".hidden.md", "a dotfile")
	write(".git/config.md", "inside a dot-directory")
	write("node_modules/pkg/readme.md", "vendored")
	// --out can point anywhere, so it is excluded by path rather than by being a dotfile.
	write("out/leftover.md", "the output directory")

	s, err := New(config.Default(), dir, "", filepath.Join(dir, "out"), "olly")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, dir
}

func getFiles(t *testing.T, s *Server) []fileEntry {
	t.Helper()

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/files", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/files = %d: %s", w.Code, w.Body)
	}
	var got []fileEntry
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode files: %v", err)
	}
	return got
}

// The explorer filters on ext and reports contributors from threads, so both have to be
// served rather than guessed from the name in the browser.
func TestFilesServesTheExplorerRow(t *testing.T) {
	s, path := newTestServer(t)

	got := getFiles(t, s)
	if len(got) != 1 {
		t.Fatalf("got %d rows; want 1", len(got))
	}
	if got[0].Rel != filepath.Base(path) || got[0].Ext != ".md" {
		t.Errorf("row = %+v", got[0])
	}
	if got[0].Threads != 0 {
		t.Errorf("threads = %d; want 0", got[0].Threads)
	}

	comment(t, s, "never push", "explain the failure mode")
	if got := getFiles(t, s); got[0].Threads != 1 {
		t.Errorf("threads = %d after commenting; want 1", got[0].Threads)
	}

	// Resolved threads contribute nothing to the payload, so naming their file as a
	// contributor would be a false alarm.
	post(t, s, "/api/thread", threadRequest{
		Rev: getDoc(t, s).Rev, ID: getDoc(t, s).Threads[0].ID, Status: "resolved",
	})
	if got := getFiles(t, s); got[0].Threads != 0 {
		t.Errorf("threads = %d with everything resolved; want 0", got[0].Threads)
	}
}

// The file-type filter keys off ext, so this pins that ext reaches nothing on the way to
// the payload: a thread in a file the default `markdown` filter hides still ships. The
// filter itself is browser state and no route accepts it, which is the other half of the
// guarantee and the half no Go test can hold — keep it that way.
func TestExtCannotChangeWhatShips(t *testing.T) {
	const page = "<h1>Report</h1>\n<p>Comments autosave to disk; never push to a terminal.</p>\n"
	s, _ := newFileServer(t, config.Default(), "report.html", page)

	if got := getFiles(t, s); got[0].Ext != ".html" {
		t.Fatalf("ext = %q; the default filter hides rows by this", got[0].Ext)
	}
	comment(t, s, "never push", "explain the failure mode")

	if got := submit(t, s); len(got.Payload.Suggestions) != 1 {
		t.Fatalf("got %d suggestions from a hidden file; want the thread to ship anyway", len(got.Payload.Suggestions))
	}
}

const htmlFixture = `<!doctype html>
<html>
<body>
<p>Comments autosave to disk; never push to a terminal.</p>
<script>alert(1)</script>
</body>
</html>
`

func TestHTMLTarget(t *testing.T) {
	s, path := newFileServer(t, config.Default(), "report.html", htmlFixture)

	d := getDoc(t, s)
	if !strings.Contains(d.HTML, "<p><span data-o=") {
		t.Errorf("the HTML target was not rendered by the HTML renderer:\n%s", d.HTML)
	}
	if strings.Contains(d.HTML, "script") || strings.Contains(d.HTML, "alert(1)") {
		t.Errorf("the script reached the page:\n%s", d.HTML)
	}

	start, end := offsetOf(t, htmlFixture, "never push")
	created := decodeDoc(t, post(t, s, "/api/anchor", anchorRequest{
		Rev: d.Rev, Start: start, End: end, Quote: "never push", Body: "say why",
	}))
	if len(created.Threads) != 1 || created.Threads[0].Quote != "never push" {
		t.Fatalf("threads = %+v", created.Threads)
	}

	onDisk := string(mustRead(t, path))
	markers := "<!--mc:a:" + created.Threads[0].ID + "-->never push<!--mc:/a:" + created.Threads[0].ID + "-->"
	if !strings.Contains(onDisk, markers) {
		t.Errorf("marker pair missing from the file:\n%s", onDisk)
	}
	// comments.Anchor refuses any span at or past the threads block, so the block has to
	// sit after everything reviewable — which for an HTML file means after </html>.
	if strings.Index(onDisk, "<!--mc:threads:begin-->") < strings.Index(onDisk, "</html>") {
		t.Errorf("threads block is not after </html>:\n%s", onDisk)
	}

	if reread := getDoc(t, s); len(reread.Threads) != 1 || reread.Threads[0].ID != created.Threads[0].ID {
		t.Errorf("re-reading the file lost the thread: %+v", reread.Threads)
	}
}

func TestDiscoversTheWholeDirectory(t *testing.T) {
	s, _ := newDirServer(t)

	var rels []string
	for _, f := range getFiles(t, s) {
		rels = append(rels, f.Rel)
	}

	want := []string{"SKILL.md", "references/api.md", "references/notes.md", "report.html"}
	if !slices.Equal(rels, want) {
		t.Errorf("files = %q; want %q", rels, want)
	}
}

// A single file is the same code path with a one-element set, which is what keeps the
// browser from needing two modes.
func TestSingleFileIsAOneElementSet(t *testing.T) {
	s, path := newTestServer(t)

	files := getFiles(t, s)
	if len(files) != 1 || files[0].Rel != "SKILL.md" {
		t.Fatalf("files = %+v", files)
	}
	if d := getDoc(t, s); d.Rel != "SKILL.md" || d.Path != path {
		t.Errorf("doc = %+v; want the target itself", d)
	}
}

func TestFileListCountsThreads(t *testing.T) {
	s, _ := newDirServer(t)
	commentOn(t, s, "references/api.md", "The reference body", "say what it returns")

	for _, f := range getFiles(t, s) {
		want := 0
		if f.Rel == "references/api.md" {
			want = 1
		}
		if f.Threads != want {
			t.Errorf("%s has %d threads; want %d", f.Rel, f.Threads, want)
		}
	}
}

func TestCommentsLandInTheFileTheyWereMadeOn(t *testing.T) {
	s, dir := newDirServer(t)

	commentOn(t, s, "references/api.md", "The reference body", "say what it returns")

	if len(getFile(t, s, "references/api.md").Threads) != 1 {
		t.Error("the comment did not land in the file it was made on")
	}
	for _, other := range []string{"SKILL.md", "references/notes.md"} {
		if len(getFile(t, s, other).Threads) != 0 {
			t.Errorf("%s picked up a comment made elsewhere", other)
		}
		if strings.Contains(string(mustRead(t, filepath.Join(dir, other))), "mc:a:") {
			t.Errorf("%s was written to on disk", other)
		}
	}
}

// A path outside the discovered set must not be readable through the file parameter.
func TestRejectsAFileOutsideTheSet(t *testing.T) {
	s, _ := newDirServer(t)

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/doc?file=../../etc/passwd", nil))

	if w.Code != http.StatusBadRequest {
		t.Errorf("status = %d; want 400", w.Code)
	}
}

func TestRevIsPerFile(t *testing.T) {
	s, dir := newDirServer(t)
	stale := getFile(t, s, "SKILL.md").Rev
	otherRev := getFile(t, s, "references/api.md").Rev

	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(fixture+"\nEdited elsewhere.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	start, end := offsetOf(t, fixture, "never push")
	w := post(t, s, "/api/anchor", anchorRequest{
		File: "SKILL.md", Rev: stale, Start: start, End: end, Quote: "never push", Body: "x",
	})
	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d; want 409 on the edited file", w.Code)
	}

	// The other file was untouched, so its rev is still good and it keeps working.
	body, bodyEnd := offsetOf(t, "# API\n\nThe reference body.\n", "The reference body")
	w = post(t, s, "/api/anchor", anchorRequest{
		File: "references/api.md", Rev: otherRev, Start: body, End: bodyEnd,
		Quote: "The reference body", Body: "still fine",
	})
	if w.Code != http.StatusOK {
		t.Errorf("an edit to one file invalidated another: %d %s", w.Code, w.Body)
	}

	// Reloading the edited file is the recovery the page performs on a 409.
	if getFile(t, s, "SKILL.md").Rev == stale {
		t.Error("rev did not move after the external edit")
	}
}

// HTML is discovered alongside Markdown, and renders through the same per-file format
// detection a single HTML target uses.
func TestHTMLFileInDirectoryRenders(t *testing.T) {
	s, _ := newDirServer(t)

	d := getFile(t, s, "report.html")
	if !strings.Contains(d.HTML, "data-o=") {
		t.Errorf("report.html did not render:\n%s", d.HTML)
	}
}

// A target with no extension, or one nobody has taught the tool about, has always been
// rendered as Markdown. Listing HTML must not have changed that.
func TestAnUnknownExtensionStillRendersAsMarkdown(t *testing.T) {
	s, _ := newFileServer(t, config.Default(), "notes.txt", fixture)

	if d := getDoc(t, s); !strings.Contains(d.HTML, `data-o="`) {
		t.Errorf("notes.txt did not render:\n%s", d.HTML)
	}
}

// Submit is per file now, so the whole review set only lands in pending.json across
// several submits — each one has to add to what an earlier submit of a different file
// left there, never replace it.
func TestHandoffAccumulatesAcrossFiles(t *testing.T) {
	s, dir := newDirServer(t)
	commentOn(t, s, "SKILL.md", "never push", "explain the failure mode")
	commentOn(t, s, "references/api.md", "The reference body", "say what it returns")
	commentOn(t, s, "references/notes.md", "Something else entirely", "cut this")

	submitFile(t, s, "SKILL.md")
	if got := len(readPendingFile(t, s).Suggestions); got != 1 {
		t.Fatalf("pending after one file = %d; want 1", got)
	}

	submitFile(t, s, "references/api.md")
	last := submitFile(t, s, "references/notes.md")

	if len(last.Payload.Suggestions) != 3 {
		t.Fatalf("got %d suggestions; want one per file", len(last.Payload.Suggestions))
	}
	var files []string
	for _, sug := range last.Payload.Suggestions {
		files = append(files, sug.File)
	}
	slices.Sort(files)
	want := []string{
		filepath.Join(dir, "SKILL.md"),
		filepath.Join(dir, "references", "api.md"),
		filepath.Join(dir, "references", "notes.md"),
	}
	slices.Sort(want)
	if !slices.Equal(files, want) {
		t.Errorf("files = %q; want %q", files, want)
	}

	// One payload, in one pending file — the skill named by the directory, not by
	// whichever document happened to be open.
	if last.Payload.SkillName != "example-skill" || last.Payload.SkillPath != dir {
		t.Errorf("payload header = %+v", last.Payload)
	}
	if len(readPendingFile(t, s).Suggestions) != 3 {
		t.Error("pending.json does not hold the whole review set")
	}
}

// The built frontend is gitignored (generated by `just web`), so this can't assert against
// whatever a developer's checkout happens to have built — that would pass or fail depending
// on build state instead of on the code. fstest.MapFS stands in for both states instead.
func TestServesThePage(t *testing.T) {
	page := []byte("<!doctype html><div id=\"root\"></div>")
	w := httptest.NewRecorder()
	assetHandler(fstest.MapFS{"index.html": {Data: page}}).
		ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET / = %d", w.Code)
	}
	if !bytes.Equal(w.Body.Bytes(), page) {
		t.Errorf("index.html not served:\n%s", w.Body)
	}
}

// A reviewed file's path is also its URL, so a deep link has to reach the SPA rather than
// the 404 a static file server would give it. ".." is checked alongside: it must land on
// the page too, never on a file outside the asset set.
func TestServesThePageForDeepLinks(t *testing.T) {
	page := []byte("<!doctype html><div id=\"root\"></div>")
	fsys := fstest.MapFS{
		"index.html": {Data: page},
		"app.js":     {Data: []byte("console.log(1)")},
	}

	for _, path := range []string{"/references/theming.md", "/SKILL.md", "/../secret"} {
		w := httptest.NewRecorder()
		assetHandler(fsys).ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))

		if w.Code != http.StatusOK {
			t.Errorf("GET %s = %d", path, w.Code)
			continue
		}
		if !bytes.Equal(w.Body.Bytes(), page) {
			t.Errorf("GET %s did not serve index.html:\n%s", path, w.Body)
		}
	}

	// The fallback must not swallow the real assets the page then asks for.
	w := httptest.NewRecorder()
	assetHandler(fsys).ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/app.js", nil))
	if got := w.Body.String(); got != "console.log(1)" {
		t.Errorf("GET /app.js = %q, want the asset", got)
	}
}

func TestServesFallbackWhenFrontendNotBuilt(t *testing.T) {
	fallback := []byte("run `just web`")
	w := httptest.NewRecorder()
	assetHandler(fstest.MapFS{"not-built.html": {Data: fallback}}).
		ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("GET / = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}
	if !bytes.Equal(w.Body.Bytes(), fallback) {
		t.Errorf("fallback page not served:\n%s", w.Body)
	}
}

// The split that is the whole feature: the file served for review and the skill the
// payload edits are two paths.
func TestHandoffNamesTheSkillNotTheTarget(t *testing.T) {
	dir := t.TempDir()
	skillDir := filepath.Join(dir, "skills", "ideate")
	if err := os.MkdirAll(skillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	report := filepath.Join(dir, "report.md")
	if err := os.WriteFile(report, []byte("# Report\n\nA claim that is wrong.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := New(config.Default(), report, skillDir, filepath.Join(dir, "out"), "olly")
	if err != nil {
		t.Fatal(err)
	}
	comment(t, s, "A claim that is wrong", "say why")

	got := submit(t, s).Payload
	if got.Mode != handoff.ModeOutput {
		t.Errorf("mode = %q; want %q", got.Mode, handoff.ModeOutput)
	}
	if got.SkillPath != skillDir || got.SkillName != "example-skill" {
		t.Errorf("payload names the target, not the skill: %+v", got)
	}
	if len(got.Suggestions) != 1 || got.Suggestions[0].File != report {
		t.Errorf("suggestion does not name the reviewed report: %+v", got.Suggestions)
	}
}
