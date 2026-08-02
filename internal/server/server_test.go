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
		Body: "say why this matters", Priority: "high", Category: "instructions",
	}))

	if len(d.Threads) != 1 {
		t.Fatalf("got %d threads; want 1", len(d.Threads))
	}
	thread := d.Threads[0]
	if thread.Quote != "never push" || thread.Priority != "high" {
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

func TestHandoff(t *testing.T) {
	s, _ := newTestServer(t)
	start, end := offsetOf(t, fixture, "never push")
	post(t, s, "/api/anchor", anchorRequest{
		Rev: getDoc(t, s).Rev, Start: start, End: end, Quote: "never push",
		Body: "explain the failure mode", Priority: "high", Category: "instructions",
	})

	w := post(t, s, "/api/handoff", struct{}{})
	if w.Code != http.StatusOK {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var got handoffResponse
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if len(got.Payload.Suggestions) != 1 {
		t.Fatalf("got %d suggestions; want 1", len(got.Payload.Suggestions))
	}
	if got.Payload.SkillName != "example-skill" || got.Payload.SkillPath != s.Path() {
		t.Errorf("payload header = %+v", got.Payload)
	}
	if !strings.Contains(got.Prompt, got.File) {
		t.Errorf("prompt %q does not name the file %q", got.Prompt, got.File)
	}

	written, err := os.ReadFile(got.File)
	if err != nil {
		t.Fatalf("payload file: %v", err)
	}
	if !bytes.Contains(written, []byte("improvement_suggestions")) {
		t.Errorf("payload file is not the skill-updater shape:\n%s", written)
	}

	// Submit is a pure function of what is already saved, so clicking twice is safe.
	if again := post(t, s, "/api/handoff", struct{}{}); again.Code != http.StatusOK {
		t.Errorf("second submit = %d; want it to be repeatable", again.Code)
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
