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

	s, err := New(path, filepath.Join(dir, "out"), "olly")
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
	if thread.Quote != "never push" || thread.Priority != "" {
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
		d := decodeDoc(t, post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: id, Category: "examples"}))
		if d.Threads[0].Category != "examples" || d.Threads[0].Status != "resolved" {
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

func submit(t *testing.T, s *Server) handoffResponse {
	t.Helper()

	w := post(t, s, "/api/handoff", struct{}{})
	if w.Code != http.StatusOK {
		t.Fatalf("handoff = %d: %s", w.Code, w.Body)
	}
	var got handoffResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode handoff: %v", err)
	}
	return got
}

func readPendingFile(t *testing.T, s *Server) pendingFile {
	t.Helper()

	var p pendingFile
	if err := json.Unmarshal(mustRead(t, filepath.Join(s.outDir, pendingName)), &p); err != nil {
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

	if err := os.Rename(filepath.Join(s.outDir, pendingName), readPendingFile(t, s).Archive); err != nil {
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
	if s := got.Payload.Suggestions[0]; s.ID == "" || s.Priority != "medium" || s.Category != "instructions" || s.ExpectedImpact == "" {
		t.Errorf("untriaged suggestion = %+v", s)
	}
	if got.Payload.SkillName != "example-skill" || got.Payload.SkillPath != s.Path() {
		t.Errorf("payload header = %+v", got.Payload)
	}
	if got.File != filepath.Join(s.outDir, pendingName) {
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
	if _, err := os.Stat(filepath.Join(s.outDir, pendingName)); !os.IsNotExist(err) {
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

	post(t, s, "/api/thread", threadRequest{Rev: getDoc(t, s).Rev, ID: id, Priority: "high"})

	got := submit(t, s)
	if got.Payload.Suggestions[0].Priority != "high" || !got.Changed {
		t.Errorf("retriage did not reach pending: %+v changed=%v", got.Payload.Suggestions[0], got.Changed)
	}
}

func TestHandoffSkipsUnchangedWrite(t *testing.T) {
	s, _ := newTestServer(t)
	comment(t, s, "never push", "explain the failure mode")
	submit(t, s)

	before, err := os.Stat(filepath.Join(s.outDir, pendingName))
	if err != nil {
		t.Fatalf("stat pending: %v", err)
	}

	got := submit(t, s)
	if got.Changed {
		t.Errorf("second submit reported a change with nothing edited")
	}
	after, err := os.Stat(filepath.Join(s.outDir, pendingName))
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
