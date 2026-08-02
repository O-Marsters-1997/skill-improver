// Package server exposes a review target — one file, or every reviewable file in a
// directory — over HTTP.
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
	"slices"
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

// target is the file or directory named on the command line; skill is what the payload
// edits, which is the same path unless --skill said otherwise.
type Server struct {
	target    string   // the file or directory named on the command line, absolute
	skill     string   // absolute
	root      string   // the directory the review set is relative to
	files     []string // the review set, relative to root, sorted
	outDir    string
	author    string
	cfg       *config.Config
	mux       *http.ServeMux
	newID     func() string
	writeFile sync.Mutex // ponytail: one lock for the whole review set; this serves one reviewer
}

// A file's extension decides its syntax once, here, because two things depend on the
// answer — which renderer runs and whether markers may be moved out of a code fence — and
// they must never disagree.
func formatOf(path string) comments.Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm":
		return comments.HTML
	default:
		return comments.Markdown
	}
}

func renderDoc(format comments.Format, src []byte) ([]byte, error) {
	if format == comments.HTML {
		return render.HTMLDoc(src)
	}
	return render.HTML(src)
}

func New(cfg *config.Config, target, skillPath, outDir, author string) (*Server, error) {
	absolute, err := filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", target, err)
	}

	absoluteSkill, err := filepath.Abs(cmp.Or(skillPath, target))
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", skillPath, err)
	}

	// -out is relative by default, so it lands in the directory the binary was run
	// from. Resolving it once here keeps the handoff prompt pasteable anywhere.
	out, err := filepath.Abs(outDir)
	if err != nil {
		return nil, fmt.Errorf("server: resolve %s: %w", outDir, err)
	}

	root, files, err := Discover(absolute, out)
	if err != nil {
		return nil, err
	}

	s := &Server{
		target: absolute, skill: absoluteSkill, root: root, files: files,
		outDir: out, author: author, cfg: cfg, mux: http.NewServeMux(), newID: comments.NewID,
	}

	assets, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, fmt.Errorf("server: embedded assets: %w", err)
	}
	s.mux.Handle("GET /", http.FileServerFS(assets))
	s.mux.HandleFunc("GET /api/files", s.handleFiles)
	s.mux.HandleFunc("GET /api/doc", s.handleDoc)
	s.mux.HandleFunc("POST /api/anchor", s.handleAnchor)
	s.mux.HandleFunc("POST /api/thread", s.handleThread)
	s.mux.HandleFunc("POST /api/thread/delete", s.handleThreadDelete)
	s.mux.HandleFunc("POST /api/handoff", s.handleHandoff)
	s.mux.HandleFunc("POST /api/file/clear", s.handleFileClear)
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// Path is the target as named on the command line, resolved: a directory when the whole
// skill is under review, the file itself when only one is.
func (s *Server) Path() string { return s.target }

func (s *Server) Skill() string { return s.skill }

// The formats a review can be held in.
var reviewable = []string{".md", ".html", ".htm"}

// Discover returns every reviewable file under target, as paths relative to root and
// sorted, so the explorer and the handoff agree on an order. A file target yields a
// one-element set, which is what keeps the two cases on one code path.
func Discover(target, outDir string) (root string, files []string, err error) {
	info, err := os.Stat(target)
	if err != nil {
		return "", nil, fmt.Errorf("server: %w", err)
	}
	if !info.IsDir() {
		return filepath.Dir(target), []string{filepath.Base(target)}, nil
	}

	err = filepath.WalkDir(target, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := entry.Name()
		if entry.IsDir() {
			// outDir is skipped by path rather than by name: it is only a dotfile by
			// default, and --out can put it anywhere.
			if path != target && (strings.HasPrefix(name, ".") || name == "node_modules" || path == outDir) {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasPrefix(name, ".") || !slices.Contains(reviewable, strings.ToLower(filepath.Ext(name))) {
			return nil
		}
		rel, err := filepath.Rel(target, path)
		if err != nil {
			return err
		}
		files = append(files, filepath.ToSlash(rel))
		return nil
	})
	if err != nil {
		return "", nil, fmt.Errorf("server: walk %s: %w", target, err)
	}
	if len(files) == 0 {
		return "", nil, fmt.Errorf("server: %s holds no reviewable files", target)
	}

	slices.Sort(files)
	return target, files, nil
}

// Docs reads a discovered review set. The Submit button and the handoff subcommand both
// go through it, so neither can hand off a different set of files from the other.
func Docs(root string, files []string) ([]handoff.Doc, error) {
	docs := make([]handoff.Doc, 0, len(files))
	for _, rel := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("server: read %s: %w", path, err)
		}
		docs = append(docs, handoff.Doc{Path: path, Src: src})
	}
	return docs, nil
}

// Every path the API accepts is matched against the set discovered at startup, so a
// crafted file= cannot reach outside the target. An empty one means the first file, which
// is what a single-file review always sends.
func (s *Server) at(rel string) (string, string, error) {
	rel = cmp.Or(rel, s.files[0])
	if !slices.Contains(s.files, rel) {
		return "", "", fmt.Errorf("server: %q is not in the review set: %w", rel, errBadRequest)
	}
	return rel, filepath.Join(s.root, filepath.FromSlash(rel)), nil
}

// Fields and Updater are served rather than baked into the page, so the browser cannot
// drift from the schema the payload is built against.
type doc struct {
	Name    string            `json:"name"`
	Rel     string            `json:"rel"`
	Path    string            `json:"path"`
	Rev     string            `json:"rev"`
	HTML    string            `json:"html"`
	Threads []comments.Thread `json:"threads"`
	Fields  []config.Field    `json:"fields"`
	Updater string            `json:"updater"`
}

// One row of the explorer. Threads counts what Build would turn into suggestions rather
// than every thread in the file, so a file whose threads are all resolved is not named as
// a contributor the Submit panel has to warn about.
type fileEntry struct {
	Rel     string `json:"rel"`
	Ext     string `json:"ext"`
	Threads int    `json:"threads"`
}

// The counts come off disk on every call rather than being cached, for the same reason
// nothing else here is: the file is the state.
func (s *Server) handleFiles(w http.ResponseWriter, _ *http.Request) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	entries := make([]fileEntry, 0, len(s.files))
	for _, rel := range s.files {
		path := filepath.Join(s.root, filepath.FromSlash(rel))
		count := 0
		if src, _, err := s.load(path); err == nil {
			threads, err := comments.Threads(src)
			if err != nil {
				writeError(w, err)
				return
			}
			count = len(handoff.Build(s.cfg, threads, "", "").Suggestions)
		}
		entries = append(entries, fileEntry{
			Rel: rel, Ext: strings.ToLower(filepath.Ext(rel)), Threads: count,
		})
	}
	writeJSON(w, http.StatusOK, entries)
}

func (s *Server) handleDoc(w http.ResponseWriter, r *http.Request) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	rel, path, err := s.at(r.URL.Query().Get("file"))
	if err != nil {
		writeError(w, err)
		return
	}
	src, rev, err := s.load(path)
	if err != nil {
		writeError(w, err)
		return
	}
	s.respond(w, rel, path, src, rev)
}

type anchorRequest struct {
	File  string `json:"file"`
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

	s.mutate(w, req.File, req.Rev, func(path string, src []byte) ([]byte, error) {
		id, err := s.freshID(src)
		if err != nil {
			return nil, err
		}
		out, anchored, err := comments.Anchor(src, req.Start, req.End, req.Quote, id, formatOf(path))
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
	File   string            `json:"file"`
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

	s.mutate(w, req.File, req.Rev, func(_ string, src []byte) ([]byte, error) {
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
	s.mutate(w, req.File, req.Rev, func(_ string, src []byte) ([]byte, error) {
		return comments.Remove(src, req.ID)
	})
}

// handleHandoff submits one file at a time: handoff.Submit merges its suggestions onto
// whatever is already pending from an earlier submit of a different file, so this cannot
// erase them. The headless `skill-review handoff` subcommand takes the whole review set
// instead and never writes back to a document — it is a read-only preview/backstop, and
// only this handler strips comments.
func (s *Server) handleHandoff(w http.ResponseWriter, r *http.Request) {
	var req fileRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}

	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	_, path, err := s.at(req.File)
	if err != nil {
		writeError(w, err)
		return
	}
	src, current, err := s.load(path)
	if err != nil {
		writeError(w, err)
		return
	}
	if req.Rev != "" && req.Rev != current {
		writeJSON(w, http.StatusConflict, map[string]string{
			"error": "the file changed on disk since you loaded it",
			"rev":   current,
		})
		return
	}

	result, err := handoff.Submit(s.cfg, s.outDir, s.skill, []handoff.Doc{{Path: path, Src: src}})
	if err != nil {
		writeError(w, err)
		return
	}
	if result.Changed {
		// The terminal is the one surface a failed clipboard write cannot take away.
		log.Printf("handoff: %s", result.Prompt)
	}

	// The payload is on disk before the threads leave the document: a save failure here
	// leaves the ids in pending.json and still in the file, and a re-submit dedupes by id.
	if len(result.Submitted) > 0 {
		out := src
		for _, id := range result.Submitted {
			if out, err = comments.Remove(out, id); err != nil {
				writeError(w, err)
				return
			}
		}
		if err := s.save(path, out); err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, result)
}

type fileRequest struct {
	File string `json:"file"`
	Rev  string `json:"rev"`
}

func (s *Server) handleFileClear(w http.ResponseWriter, r *http.Request) {
	var req fileRequest
	if err := decode(r, &req); err != nil {
		writeError(w, err)
		return
	}
	s.mutate(w, req.File, req.Rev, func(_ string, src []byte) ([]byte, error) {
		return comments.Clear(src), nil
	})
}

// The write is refused if the file changed since the client last read it. The rev is per
// file, so an edit made in the editor to one document leaves the rest of the set usable.
func (s *Server) mutate(w http.ResponseWriter, file, rev string, fn func(path string, src []byte) ([]byte, error)) {
	s.writeFile.Lock()
	defer s.writeFile.Unlock()

	rel, path, err := s.at(file)
	if err != nil {
		writeError(w, err)
		return
	}
	src, current, err := s.load(path)
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

	out, err := fn(path, src)
	if err != nil {
		writeError(w, err)
		return
	}
	if err := s.save(path, out); err != nil {
		writeError(w, err)
		return
	}

	_, current, err = s.load(path)
	if err != nil {
		writeError(w, err)
		return
	}
	s.respond(w, rel, path, out, current)
}

func (s *Server) respond(w http.ResponseWriter, rel, path string, src []byte, rev string) {
	html, err := renderDoc(formatOf(path), src)
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
		// The heading names the skill, not the file being read: a directory review has
		// many documents and only one of them carries the frontmatter.
		Name:    skill.NameAt(s.skill),
		Rel:     rel,
		Path:    path,
		Rev:     rev,
		HTML:    string(html),
		Threads: threads,
		Fields:  s.cfg.Fields,
		Updater: s.cfg.Updater.Name,
	})
}

// The revision stamp is what lets a simultaneous edit in the editor be caught instead
// of silently overwritten.
func (s *Server) load(path string) ([]byte, string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, "", fmt.Errorf("server: read %s: %w", path, err)
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, "", fmt.Errorf("server: stat %s: %w", path, err)
	}
	return src, strconv.FormatInt(info.ModTime().UnixNano(), 36) + "-" + strconv.FormatInt(info.Size(), 36), nil
}

// Writing via a temporary file in the same directory means an interrupted write can
// never leave a half-written document behind.
func (s *Server) save(path string, src []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("server: stat %s: %w", path, err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), ".skill-review-*")
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
	if err := os.Rename(tmp.Name(), path); err != nil {
		return fmt.Errorf("server: replace %s: %w", path, err)
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
