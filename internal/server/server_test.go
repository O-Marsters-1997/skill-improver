package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

// The fixture is copied so a test can mutate it freely.
func newTestServer(t *testing.T) (*Server, string) {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	s, err := New(config.Default(), path, "", filepath.Join(dir, "out"), "olly")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return s, path
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

func getDoc(t *testing.T, s *Server) doc {
	t.Helper()

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/doc", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/doc = %d: %s", w.Code, w.Body)
	}
	var d doc
	if err := json.Unmarshal(w.Body.Bytes(), &d); err != nil {
		t.Fatalf("decode doc: %v", err)
	}
	return d
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
	dir := t.TempDir()
	path := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(path, []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Fields:  []config.Field{{Name: "severity", Label: "How bad", Values: []string{"blocker", "nit"}, Default: "nit"}},
		Updater: config.Updater{Name: "my-updater"},
	}
	s, err := New(cfg, path, "", filepath.Join(dir, "out"), "olly")
	if err != nil {
		t.Fatal(err)
	}

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

func comment(t *testing.T, s *Server, passage, body string) {
	t.Helper()

	start, end := offsetOf(t, string(mustRead(t, s.Path())), passage)
	w := post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: passage, Body: body,
	})
	if w.Code != http.StatusOK {
		t.Fatalf("anchor %q = %d: %s", passage, w.Code, w.Body)
	}
}

func submit(t *testing.T, s *Server) handoff.Result {
	t.Helper()

	w := post(t, s, "/api/handoff", struct{}{})
	if w.Code != http.StatusOK {
		t.Fatalf("handoff = %d: %s", w.Code, w.Body)
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

func TestHandoffReflectsRetriage(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	id := submit(t, s).Payload.Suggestions[0].ID

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

func TestServesThePage(t *testing.T) {
	s, _ := newTestServer(t)

	w := httptest.NewRecorder()
	s.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("GET / = %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), `id="doc"`) {
		t.Errorf("index.html not served:\n%s", w.Body)
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
