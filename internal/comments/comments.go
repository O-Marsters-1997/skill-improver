// Package comments reads and writes review threads stored inline in a Markdown
// file, using the "mc" marker format: an anchored passage is wrapped in
// `<!--mc:a:ID-->text<!--mc:/a:ID-->`, and every thread occupies one
// `<!--mc:t {JSON}-->` line inside a block at the end of the file.
//
// Storing comments in the document means the anchor survives edits and nothing is
// held in memory: the file on disk is the only state.
package comments

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"regexp"
	"strconv"
)

const (
	threadsBegin = "<!--mc:threads:begin-->"
	threadsEnd   = "<!--mc:threads:end-->"
	threadPrefix = "<!--mc:t "
	threadSuffix = "-->"

	snapWindow = 64
)

var (
	ErrRange       = errors.New("comments: offsets out of range")
	ErrBadID       = errors.New("comments: id must be 1-12 base36 characters")
	ErrDuplicateID = errors.New("comments: id already anchored in this document")
	ErrInThreads   = errors.New("comments: span overlaps the threads block")
	ErrOverlap     = errors.New("comments: span partially overlaps an existing anchor")
	ErrNotFound    = errors.New("comments: thread not found")
)

// Field order matches the mc spec so the serialised line stays diff-stable.
type Comment struct {
	ID       string `json:"id"`
	Parent   string `json:"parent,omitempty"`
	Author   string `json:"author"`
	TS       string `json:"ts"`
	Body     string `json:"body"`
	EditedTS string `json:"editedTs,omitempty"`
	Deleted  bool   `json:"deleted,omitempty"`
}

// Fields and Impact are extensions to the mc format. Fields holds whatever triage the
// config asks for — priority and category, by default — captured while commenting so the
// handoff payload is complete before Submit is clicked. They are written flat onto the
// thread object rather than nested, so a file written before the schema was configurable
// still reads, and other tools that read mc ignore the keys they do not know.
type Thread struct {
	ID       string
	Quote    string
	Status   string // "open" or "resolved"
	Comments []Comment
	Fields   map[string]string
	Impact   string
}

// The fixed keys, in mc order. Everything dynamic is appended after them.
type threadHead struct {
	ID       string    `json:"id"`
	Quote    string    `json:"quote"`
	Status   string    `json:"status"`
	Comments []Comment `json:"comments"`
}

func (t Thread) MarshalJSON() ([]byte, error) {
	head, err := json.Marshal(threadHead{t.ID, t.Quote, t.Status, t.Comments})
	if err != nil {
		return nil, err
	}

	tail := make(map[string]string, len(t.Fields)+1)
	for name, value := range t.Fields {
		if value != "" {
			tail[name] = value
		}
	}
	if t.Impact != "" {
		tail["impact"] = t.Impact
	}
	if len(tail) == 0 {
		return head, nil
	}

	// Both halves are objects, and encoding/json writes map keys in sorted order, so
	// splicing them keeps the line deterministic and therefore diff-stable.
	rest, err := json.Marshal(tail)
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	b.Write(head[:len(head)-1])
	b.WriteByte(',')
	b.Write(rest[1:])
	return b.Bytes(), nil
}

// Any string key that is not one of the fixed ones is a configured field. Keeping them
// means a field removed from the config is preserved rather than erased from the file the
// next time an unrelated thread is written.
func (t *Thread) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	known := map[string]any{
		"id": &t.ID, "quote": &t.Quote, "status": &t.Status,
		"comments": &t.Comments, "impact": &t.Impact,
	}
	for key, into := range known {
		if value, ok := raw[key]; ok {
			if err := json.Unmarshal(value, into); err != nil {
				return err
			}
			delete(raw, key)
		}
	}

	for key, value := range raw {
		var s string
		if json.Unmarshal(value, &s) != nil {
			continue
		}
		if t.Fields == nil {
			t.Fields = map[string]string{}
		}
		t.Fields[key] = s
	}
	return nil
}

var (
	markerPattern = regexp.MustCompile(`<!--mc:(/?)a:([a-z0-9]{1,12})-->`)
	threadPattern = regexp.MustCompile(`(?m)^<!--mc:t (.*)-->[ \t]*$`)
)

// A document with no threads block is not an error, it simply has no comments.
func Threads(src []byte) ([]Thread, error) {
	block, _, _, ok := threadsBlock(src)
	if !ok {
		return nil, nil
	}

	matches := threadPattern.FindAllSubmatch(block, -1)
	threads := make([]Thread, 0, len(matches))
	for _, m := range matches {
		var t Thread
		if err := json.Unmarshal(m[1], &t); err != nil {
			return nil, fmt.Errorf("comments: corrupt thread line %q: %w", m[0], err)
		}
		threads = append(threads, t)
	}
	return threads, nil
}

// quote is the reviewer's selection as the browser reported it, used only as a
// hint: an exact match confirms the offsets, a near miss snaps to them, and no
// match at all leaves the offsets to stand. It is deliberately not authoritative —
// a selection spanning several rendered runs ("the **lazy** fix" reads as "the lazy
// fix") can never equal the source slice, and staleness is already caught upstream
// by the revision check.
func Anchor(src []byte, start, end int, quote, id string) ([]byte, string, error) {
	if !validID(id) {
		return nil, "", ErrBadID
	}
	if start < 0 || end > len(src) || start >= end {
		return nil, "", ErrRange
	}
	if bytes.Contains(src, []byte(openMarker(id))) {
		return nil, "", ErrDuplicateID
	}

	start, end = snap(src, start, end, quote)
	start, end, block := expandFences(src, start, end)

	if i := bytes.Index(src, []byte(threadsBegin)); i >= 0 && end > i {
		return nil, "", ErrInThreads
	}
	if err := checkOverlap(src, start, end); err != nil {
		return nil, "", err
	}

	opening, closing := openMarker(id), closeMarker(id)
	if block {
		opening, closing = opening+"\n", "\n"+closing
	}

	out := make([]byte, 0, len(src)+len(opening)+len(closing))
	out = append(out, src[:start]...)
	out = append(out, opening...)
	out = append(out, src[start:end]...)
	out = append(out, closing...)
	out = append(out, src[end:]...)
	return out, string(src[start:end]), nil
}

// Only the target thread's line is rewritten, so unknown fields on other threads
// survive untouched.
func Upsert(src []byte, t Thread) ([]byte, error) {
	line, err := threadLine(t)
	if err != nil {
		return nil, err
	}

	block, blockStart, blockEnd, ok := threadsBlock(src)
	if !ok {
		out := bytes.TrimRight(src, "\n")
		return append(out, "\n\n"+threadsBegin+"\n"+line+"\n"+threadsEnd+"\n"...), nil
	}

	if loc := findThreadLine(block, t.ID); loc != nil {
		return splice(src, blockStart+loc[0], blockStart+loc[1], line), nil
	}
	return splice(src, blockEnd, blockEnd, line+"\n"), nil
}

func Remove(src []byte, id string) ([]byte, error) {
	if !validID(id) {
		return nil, ErrBadID
	}

	out := src
	block, blockStart, _, ok := threadsBlock(out)
	if !ok {
		return nil, ErrNotFound
	}
	loc := findThreadLine(block, id)
	if loc == nil {
		return nil, ErrNotFound
	}
	// Take the newline with the line so the block does not accumulate blanks.
	out = splice(out, blockStart+loc[0], min(blockStart+loc[1]+1, len(out)), "")

	// Block-mode markers were written on their own line; drop that line whole.
	for _, marker := range []string{openMarker(id) + "\n", "\n" + closeMarker(id), openMarker(id), closeMarker(id)} {
		out = bytes.Replace(out, []byte(marker), nil, 1)
	}
	return out, nil
}

func NewID() string {
	return strconv.FormatUint(rand.Uint64N(36*36*36*36*36*36), 36)
}

func openMarker(id string) string  { return "<!--mc:a:" + id + "-->" }
func closeMarker(id string) string { return "<!--mc:/a:" + id + "-->" }

func validID(id string) bool {
	if len(id) == 0 || len(id) > 12 {
		return false
	}
	for i := range len(id) {
		if !isBase36(id[i]) {
			return false
		}
	}
	return true
}

func isBase36(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z'
}

func threadsBlock(src []byte) (block []byte, start, end int, ok bool) {
	i := bytes.Index(src, []byte(threadsBegin))
	if i < 0 {
		return nil, 0, 0, false
	}
	start = i + len(threadsBegin) + 1 // skip the newline after the delimiter
	if start > len(src) {
		return nil, 0, 0, false
	}
	j := bytes.Index(src[start:], []byte(threadsEnd))
	if j < 0 {
		return nil, 0, 0, false
	}
	end = start + j
	return src[start:end], start, end, true
}

func findThreadLine(block []byte, id string) []int {
	for _, loc := range threadPattern.FindAllSubmatchIndex(block, -1) {
		var t Thread
		if err := json.Unmarshal(block[loc[2]:loc[3]], &t); err == nil && t.ID == id {
			return []int{loc[0], loc[1]}
		}
	}
	return nil
}

func threadLine(t Thread) (string, error) {
	if t.Comments == nil {
		t.Comments = []Comment{}
	}
	// Default HTML escaping is load-bearing: it turns a "-->" in a comment body
	// into >, which would otherwise close the marker early.
	encoded, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("comments: encode thread %q: %w", t.ID, err)
	}
	return threadPrefix + string(encoded) + threadSuffix, nil
}

func splice(src []byte, start, end int, with string) []byte {
	out := make([]byte, 0, len(src)-(end-start)+len(with))
	out = append(out, src[:start]...)
	out = append(out, with...)
	return append(out, src[end:]...)
}

func snap(src []byte, start, end int, quote string) (int, int) {
	q := []byte(quote)
	if len(q) == 0 || bytes.Equal(src[start:end], q) {
		return start, end
	}
	lo := max(0, start-snapWindow)
	hi := min(len(src), start+len(q)+snapWindow)
	if i := bytes.Index(src[lo:hi], q); i >= 0 {
		return lo + i, lo + i + len(q)
	}
	return start, end
}

// A span touching a fenced code block is grown to contain the whole fence, so markers
// are never inserted into code. block reports that the span grew, and so that the
// markers need lines of their own.
func expandFences(src []byte, start, end int) (int, int, bool) {
	block := false
	for _, f := range fences(src) {
		if end <= f[0] || start >= f[1] {
			continue
		}
		if start > f[0] || end < f[1] {
			block = true
		}
		start, end = min(start, f[0]), max(end, f[1])
	}
	return start, end, block
}

var fencePattern = regexp.MustCompile("(?m)^[ \t]{0,3}(```+|~~~+)")

// Ranges run from the start of the opening delimiter line to the end of the closing one.
func fences(src []byte) [][2]int {
	locs := fencePattern.FindAllSubmatchIndex(src, -1)
	var out [][2]int
	for i := 0; i+1 < len(locs); i += 2 {
		opening, closing := locs[i], locs[i+1]
		end := closing[1]
		if nl := bytes.IndexByte(src[end:], '\n'); nl >= 0 {
			end += nl
		} else {
			end = len(src)
		}
		out = append(out, [2]int{opening[0], end})
	}
	if len(locs)%2 == 1 { // unterminated fence runs to EOF
		out = append(out, [2]int{locs[len(locs)-1][0], len(src)})
	}
	return out
}

// Only a span that cuts through an existing marker pair is rejected: fully nested and
// fully containing spans are both fine.
func checkOverlap(src []byte, start, end int) error {
	pending := make(map[string][]int)
	for _, loc := range markerPattern.FindAllSubmatchIndex(src, -1) {
		id := string(src[loc[4]:loc[5]])
		if loc[2] == loc[3] { // the "/" group is empty, so this opens a pair
			pending[id] = loc
			continue
		}
		o, ok := pending[id]
		if !ok {
			continue
		}
		delete(pending, id)

		outside := end <= o[0] || start >= loc[1]
		inside := start >= o[1] && end <= loc[0]
		around := start <= o[0] && end >= loc[1]
		if !outside && !inside && !around {
			return ErrOverlap
		}
	}
	return nil
}
