// Package server exposes one SKILL.md for review over HTTP.
//
// Every mutation is a read-modify-write of the file on disk, and nothing is held in
// memory between requests. The comment tools this replaces lose work by pushing text
// into a terminal or the clipboard, where it drops silently. Here a failed request
// loses nothing, because the file already holds the state.
package server

import (
	"cmp"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/O-Marsters-1997/improve-skills/internal/comments"
	"github.com/O-Marsters-1997/improve-skills/internal/config"
	"github.com/O-Marsters-1997/improve-skills/internal/handoff"
	"github.com/O-Marsters-1997/improve-skills/internal/render"
	"github.com/O-Marsters-1997/improve-skills/internal/skill"
)

//go:embed web
var webFS embed.FS

// path is the document under review; skill is what the payload edits, which is the same
// path unless --skill said otherwise.
type Server struct {
	path      string
	skill     string
	outDir    string
	author    string
	cfg       *config.Config
	mux       *http.ServeMux
	newID     func() string
	writeFile sync.Mutex // ponytail: one lock for the whole file; this serves one reviewer
}

func New(cfg *config.Config, path, skillPath, outDir, author string) (*Server, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", path, err)
	}
	if _, err := os.ReadFile(absolute); err != nil {
		return nil, fmt.Errorf("server: %w", err)
	}

	absoluteSkill, err := filepath.Abs(cmp.Or(skillPath, path))
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", skillPath, err)
	}

	// -out is relative by default, so it lands in the directory the binary was run
	// from. Resolving it once here keeps the handoff prompt pasteable anywhere.
	out, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", outDir, err)
	}

	s := &Server{path: absolute, skill: absoluteSkill, outDir: out, author: author, cfg: cfg, mux: http.NewServeMux(), newID: comments.NewID}

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

func (s *Server) Skill() string { return s.skill }

// Fields and Updater are served rather than baked into the page, so the browser cannot
// drift from the schema the payload is built against.
type doc struct {
	Name    string            `json:"name"`
	Path    string            `json:"path"`
	Rev     string            `json:"rev"`
	HTML    string            `json:"html"`
	Threads []comments.Thread `json:"threads"`
	Fields  []config.Field    `json:"fields"`
	Updater string            `json:"updater"`
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
		id, err := s.freshID(src)
		if err != nil {
			return nil, err
		}
		out, anchored, err := comments.Anchor(src, req.Start, req.End, req.Quote, id)
		if err != nil {
			return nil, err
		}
		// The triage fields are left unset: they are comparative judgements, made
		// on the thread cards once there is something to compare against.
		return comments.Upsert(out, comments.Thread{
			ID:       id,
			Quote:    anchored,
			Status:   "open",
			Comments: []comments.Comment{s.newComment("c1", "", req.Body)},
		})
	})
}

// Ids are random, and handoff.Submit drops any suggestion whose id is already archived, so
// a redrawn id would make the new thread vanish from the payload without a word. Retrying
// is invisible to the reviewer; the attempt limit only stops a broken id source spinning
// the request forever, since 36^6 ids against a handful in use never needs a second draw.
func (s *Server) freshID(src []byte) (string, error) {
	used := handoff.ArchivedIDs(s.outDir)
	maps.Copy(used, comments.IDs(src))
	for range 10 {
		if id := s.newID(); !used[id] {
			return id, nil
		}
	}
	return "", errNoFreshID
}

type threadRequest struct {
	Rev    string            `json:"rev"`
	ID     string            `json:"id"`
	Body   string            `json:"body"`
	Status string            `json:"status"`
	Fields map[string]string `json:"fields"`
	Impact string            `json:"impact"`
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
			setIfGiven(&t.Impact, req.Impact)
			// Only configured fields are accepted, so a stale page cannot write a
			// key the payload will not carry.
			for name, value := range req.Fields {
				if _, ok := s.cfg.Field(name); !ok || value == "" {
					continue
				}
				if t.Fields == nil {
					t.Fields = map[string]string{}
				}
				t.Fields[name] = value
			}
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

// Everything below the lock lives in handoff.Submit, so the browser's Submit button and
// the handoff subcommand cannot produce different payloads.
func (s *Server) handleHandoff(w http.ResponseWriter, r *http.Request) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	src, _, err := s.load()
	if err != nil {
		writeError(w, err)
		return
	}

	result, err := handoff.Submit(s.cfg, s.outDir, s.skill, []handoff.Doc{{Path: s.path, Src: src}})
	if err != nil {
		writeError(w, err)
		return
	}
	if result.Changed {
		// The terminal is the one surface a failed clipboard write cannot take away.
		log.Printf("handoff: %s", result.Prompt)
	}
	writeJSON(w, http.StatusOK, result)
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
		Name:    skill.Name(src),
		Path:    s.path,
		Rev:     rev,
		HTML:    string(html),
		Threads: threads,
		Fields:  s.cfg.Fields,
		Updater: s.cfg.Updater.Name,
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

var (
	errBadRequest = errors.New("malformed request")
	errNoFreshID  = errors.New("server: every id offered is already in use")
)

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
