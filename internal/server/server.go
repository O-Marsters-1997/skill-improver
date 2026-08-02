// Package server exposes one SKILL.md for review over HTTP.
//
// Every mutation is a read-modify-write of the file on disk, and nothing is held in
// memory between requests. The comment tools this replaces lose work by pushing text
// into a terminal or the clipboard, where it drops silently. Here a failed request
// loses nothing, because the file already holds the state.
package server

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
	"github.com/O-Marsters-1997/improve-skills/internal/handoff"
	"github.com/O-Marsters-1997/improve-skills/internal/render"
)

//go:embed web
var webFS embed.FS

type Server struct {
	path      string
	outDir    string
	author    string
	mux       *http.ServeMux
	writeFile sync.Mutex // ponytail: one lock for the whole file; this serves one reviewer
}

func New(path, outDir, author string) (*Server, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", path, err)
	}
	if _, err := os.ReadFile(absolute); err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	// -out is relative by default, so it lands in the directory the binary was run
	// from. Resolving it once here keeps the handoff prompt pasteable anywhere.
	out, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", outDir, err)
	}

	s := &Server{path: absolute, outDir: out, author: author, mux: http.NewServeMux()}

	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, fmt.Errorf("server: embedded assets: %w", err)
	}
	s.mux.Handle("GET /", http.FileServerFS(assets))
	s.mux.HandleFunc("GET /api/doc", s.handleDoc)
	s.mux.HandleFunc("POST /api/anchor", s.handleAnchor)
	s.mux.HandleFunc("POST /api/thread", s.handleThread)
	s.mux.HandleFunc("POST /api/thread/delete", s.handleThreadDelete)
	s.mux.HandleFunc("POST /api/handoff", s.handleHandoff)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

func (s *Server) Path() string { return s.path }

type doc struct {
	Name    string            `json:"name"`
	Path    string            `json:"path"`
	Rev     string            `json:"rev"`
	HTML    string            `json:"html"`
	Threads []comments.Thread `json:"threads"`
}

func (s *Server) handleDoc(w http.ResponseWriter, r *http.Request) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	src, rev, err := s.load()
	if err != nil {
		writeError(w, err)
		return
	}
	s.respond(w, src, rev)
}

type anchorRequest struct {
	Rev   string `json:"rev"`
	Start int    `json:"start"`
	End   int    `json:"end"`
	Quote string `json:"quote"`
	Body  string `json:"body"`
}

func (s *Server) handleAnchor(w http.ResponseWriter, r *http.Request) {
	var req anchorRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	s.mutate(w, req.Rev, func(src []byte) ([]byte, error) {
		id := comments.NewID()
		out, anchored, err := comments.Anchor(src, req.Start, req.End, req.Quote, id)
		if err != nil {
			return nil, err
		}
		// Priority and category are left unset: they are comparative judgements,
		// made on the thread cards once there is something to compare against.
		return comments.Upsert(out, comments.Thread{
			ID:       id,
			Quote:    anchored,
			Status:   "open",
			Comments: []comments.Comment{s.newComment("c1", "", req.Body)},
		})
	})
}

type threadRequest struct {
	Rev      string `json:"rev"`
	ID       string `json:"id"`
	Body     string `json:"body"`
	Status   string `json:"status"`
	Priority string `json:"priority"`
	Category string `json:"category"`
	Impact   string `json:"impact"`
}

// Fields are patched, not replaced: an empty field in the request is left alone.
func (s *Server) handleThread(w http.ResponseWriter, r *http.Request) {
	var req threadRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	s.mutate(w, req.Rev, func(src []byte) ([]byte, error) {
		threads, err := comments.Threads(src)
		if err != nil {
			return nil, err
		}
		for _, t := range threads {
			if t.ID != req.ID {
				continue
			}
			if body := strings.TrimSpace(req.Body); body != "" {
				id, parent := nextCommentID(t)
				t.Comments = append(t.Comments, s.newComment(id, parent, body))
			}
			setIfGiven(&t.Status, req.Status)
			setIfGiven(&t.Priority, req.Priority)
			setIfGiven(&t.Category, req.Category)
			setIfGiven(&t.Impact, req.Impact)
			return comments.Upsert(src, t)
		}
		return nil, fmt.Errorf("server: thread %q: %w", req.ID, comments.ErrNotFound)
	})
}

func (s *Server) handleThreadDelete(w http.ResponseWriter, r *http.Request) {
	var req threadRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	s.mutate(w, req.Rev, func(src []byte) ([]byte, error) {
		return comments.Remove(src, req.ID)
	})
}

type handoffResponse struct {
	File    string          `json:"file"`
	Prompt  string          `json:"prompt"`
	Changed bool            `json:"changed"`
	Payload handoff.Payload `json:"payload"`
}

// The embedded payload flattens, so the file stays a superset of what skill-updater
// reads: the prompt and its archive target ride along rather than living only in a
// browser toast.
type pendingFile struct {
	handoff.Payload
	Prompt  string `json:"prompt"`
	Archive string `json:"archive"`
}

const pendingName = "pending.json"

// Pending is regenerated from the document every time rather than appended to, so a
// thread that was retriaged, replied to, resolved or deleted since the last submit is
// reflected. Only threads already moved to an archive file are excluded, which is what
// stops skill-updater being handed the same suggestion twice.
func (s *Server) handleHandoff(w http.ResponseWriter, r *http.Request) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	src, _, err := s.load()
	if err != nil {
		writeError(w, err)
		return
	}
	threads, err := comments.Threads(src)
	if err != nil {
		writeError(w, err)
		return
	}

	if err := os.MkdirAll(s.outDir, 0o755); err != nil {
		writeError(w, fmt.Errorf("server: create %s: %w", s.outDir, err))
		return
	}

	file := filepath.Join(s.outDir, pendingName)
	archived := s.archivedIDs()
	payload := handoff.Build(threads, handoff.SkillName(src), s.path)
	payload.Suggestions = slices.DeleteFunc(payload.Suggestions, func(sug handoff.Suggestion) bool {
		return archived[sug.ID]
	})

	if len(payload.Suggestions) == 0 {
		if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
			writeError(w, fmt.Errorf("server: remove %s: %w", file, err))
			return
		}
		writeJSON(w, http.StatusOK, handoffResponse{Payload: payload})
		return
	}

	// Reusing the archive name already recorded keeps a prompt copied earlier valid,
	// and lets an unchanged pending file compare equal.
	archive := s.readPending().Archive
	if archive == "" {
		archive = filepath.Join(s.outDir, fmt.Sprintf("handoff-%s-%s.json",
			or(payload.SkillName, "skill"), time.Now().UTC().Format("20060102T150405Z")))
	}
	prompt := fmt.Sprintf(
		"Use skill-updater with the payload in %s\n\n"+
			"Once applied, archive it so these suggestions are not handed off again:\nmv %s %s",
		file, file, archive)

	body, err := json.MarshalIndent(pendingFile{Payload: payload, Prompt: prompt, Archive: archive}, "", "  ")
	if err != nil {
		writeError(w, fmt.Errorf("server: encode payload: %w", err))
		return
	}

	changed := true
	if existing, err := os.ReadFile(file); err == nil {
		changed = !bytes.Equal(existing, body)
	}
	if changed {
		if err := os.WriteFile(file, body, 0o644); err != nil {
			writeError(w, fmt.Errorf("server: write %s: %w", file, err))
			return
		}
		// The terminal is the one surface a failed clipboard write cannot take away.
		log.Printf("handoff: %s", prompt)
	}

	writeJSON(w, http.StatusOK, handoffResponse{File: file, Prompt: prompt, Changed: changed, Payload: payload})
}

func (s *Server) readPending() pendingFile {
	var p pendingFile
	if body, err := os.ReadFile(filepath.Join(s.outDir, pendingName)); err == nil {
		_ = json.Unmarshal(body, &p)
	}
	return p
}

// Every thread already moved into an archive file. One that will not decode is skipped
// rather than fatal — but noisily, since ignoring it silently would let an
// already-applied suggestion come back.
func (s *Server) archivedIDs() map[string]bool {
	ids := map[string]bool{}
	entries, err := filepath.Glob(filepath.Join(s.outDir, "*.json"))
	if err != nil {
		log.Printf("handoff: cannot list %s: %v", s.outDir, err)
		return ids
	}
	for _, entry := range entries {
		if filepath.Base(entry) == pendingName {
			continue
		}
		var payload handoff.Payload
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

// The write is refused if the file changed since the client last read it.
func (s *Server) mutate(w http.ResponseWriter, rev string, fn func([]byte) ([]byte, error)) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	src, current, err := s.load()
	if err != nil {
		writeError(w, err)
		return
	}
	if rev != "" && rev != current {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "the file changed on disk since you loaded it",
			"rev":   current,
		})
		return
	}

	out, err := fn(src)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.save(out); err != nil {
		writeError(w, err)
		return
	}

	_, current, err = s.load()
	if err != nil {
		writeError(w, err)
		return
	}
	s.respond(w, out, current)
}

func (s *Server) respond(w http.ResponseWriter, src []byte, rev string) {
	html, err := render.HTML(src)
	if err != nil {
		writeError(w, err)
		return
	}
	threads, err := comments.Threads(src)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, doc{
		Name:    handoff.SkillName(src),
		Path:    s.path,
		Rev:     rev,
		HTML:    string(html),
		Threads: threads,
	})
}

// The revision stamp is what lets a simultaneous edit in the editor be caught instead
// of silently overwritten.
func (s *Server) load() ([]byte, string, error) {
	src, err := os.ReadFile(s.path)
	if err != nil {
		return nil, "", fmt.Errorf("server: read %s: %w", s.path, err)
	}
	info, err := os.Stat(s.path)
	if err != nil {
		return nil, "", fmt.Errorf("server: stat %s: %w", s.path, err)
	}
	return src, strconv.FormatInt(info.ModTime().UnixNano(), 36) + "-" + strconv.FormatInt(info.Size(), 36), nil
}

// Writing via a temporary file in the same directory means an interrupted write can
// never leave a half-written SKILL.md behind.
func (s *Server) save(src []byte) error {
	info, err := os.Stat(s.path)
	if err != nil {
		return fmt.Errorf("server: stat %s: %w", s.path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".skill-review-*")
	if err != nil {
		return fmt.Errorf("server: create temp file: %w", err)
	}
	defer os.Remove(tmp.Name())

	if _, err := tmp.Write(src); err != nil {
		tmp.Close()
		return fmt.Errorf("server: write temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("server: close temp file: %w", err)
	}
	if err := os.Chmod(tmp.Name(), info.Mode()); err != nil {
		return fmt.Errorf("server: chmod temp file: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("server: replace %s: %w", s.path, err)
	}
	return nil
}

func (s *Server) newComment(id, parent, body string) comments.Comment {
	return comments.Comment{
		ID:     id,
		Parent: parent,
		Author: s.author,
		TS:     time.Now().UTC().Format(time.RFC3339),
		Body:   body,
	}
}

func nextCommentID(t comments.Thread) (id, parent string) {
	highest := 0
	for _, c := range t.Comments {
		if n, err := strconv.Atoi(strings.TrimPrefix(c.ID, "c")); err == nil && n > highest {
			highest = n
		}
		if !c.Deleted {
			parent = c.ID
		}
	}
	return "c" + strconv.Itoa(highest+1), parent
}

func decode(r *http.Request, into any) error {
	if err := json.NewDecoder(r.Body).Decode(into); err != nil {
		return fmt.Errorf("server: %w: %v", errBadRequest, err)
	}
	return nil
}

var errBadRequest = errors.New("malformed request")

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	switch {
	case errors.Is(err, comments.ErrNotFound):
		status = http.StatusNotFound
	case errors.Is(err, errBadRequest):
		status = http.StatusBadRequest
	case errors.Is(err, comments.ErrRange), errors.Is(err, comments.ErrOverlap),
		errors.Is(err, comments.ErrInThreads), errors.Is(err, comments.ErrBadID),
		errors.Is(err, comments.ErrDuplicateID):
		status = http.StatusUnprocessableEntity
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func setIfGiven(field *string, value string) {
	if value != "" {
		*field = value
	}
}

func or(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
