package comments

import (
	"errors"
	"strings"
	"testing"
)

const fence = "```"

func TestAnchor(t *testing.T) {
	doc := "Comments autosave to disk; never push to a terminal.\n"
	offsetOf := func(src, sub string) (int, int) {
		t.Helper()
		i := strings.Index(src, sub)
		if i < 0 {
			t.Fatalf("substring %q not in source", sub)
		}
		return i, i + len(sub)
	}

	t.Run("exact offsets wrap exactly the quote", func(t *testing.T) {
		start, end := offsetOf(doc, "never push")
		got, anchored, err := Anchor([]byte(doc), start, end, "never push", "k3f")
		if err != nil {
			t.Fatalf("Anchor: %v", err)
		}
		want := "Comments autosave to disk; <!--mc:a:k3f-->never push<!--mc:/a:k3f--> to a terminal.\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
		if anchored != "never push" {
			t.Errorf("anchored = %q; want %q", anchored, "never push")
		}
	})

	t.Run("drifted offsets snap to the quote", func(t *testing.T) {
		start, end := offsetOf(doc, "never push")
		got, anchored, err := Anchor([]byte(doc), start-9, end-9, "never push", "k3f")
		if err != nil {
			t.Fatalf("Anchor: %v", err)
		}
		if !strings.Contains(string(got), "<!--mc:a:k3f-->never push<!--mc:/a:k3f-->") {
			t.Errorf("did not snap to quote:\n%s", got)
		}
		if anchored != "never push" {
			t.Errorf("anchored = %q; want %q", anchored, "never push")
		}
	})

	// A selection spanning several rendered text runs ("never push a terminal" as the
	// browser reports it) can never equal the source slice. The offsets are still right,
	// so they win and the caller is told what was actually wrapped.
	t.Run("unmatchable quote falls back to the offsets", func(t *testing.T) {
		start, end := offsetOf(doc, "never push to a terminal")
		_, anchored, err := Anchor([]byte(doc), start, end, "never push terminal", "k3f")
		if err != nil {
			t.Fatalf("Anchor: %v", err)
		}
		if anchored != "never push to a terminal" {
			t.Errorf("anchored = %q; want the source slice", anchored)
		}
	})

	t.Run("rejects bad input", func(t *testing.T) {
		tests := []struct {
			name       string
			src        string
			start, end int
			quote, id  string
			want       error
		}{
			{"inverted range", doc, 10, 10, "x", "k3f", ErrRange},
			{"negative start", doc, -1, 5, "Comme", "k3f", ErrRange},
			{"end past EOF", doc, 0, len(doc) + 1, "x", "k3f", ErrRange},
			{"empty id", doc, 0, 8, "Comments", "", ErrBadID},
			{"non-base36 id", doc, 0, 8, "Comments", "k 3f", ErrBadID},
			{"duplicate id", "a <!--mc:a:k3f-->b<!--mc:/a:k3f--> c", 0, 1, "a", "k3f", ErrDuplicateID},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				_, _, err := Anchor([]byte(tt.src), tt.start, tt.end, tt.quote, tt.id)
				if !errors.Is(err, tt.want) {
					t.Errorf("got %v; want %v", err, tt.want)
				}
			})
		}
	})

	t.Run("refuses to anchor inside the threads block", func(t *testing.T) {
		src := "text\n\n" + threadsBegin + "\n" + threadsEnd + "\n"
		start, end := offsetOf(src, threadsBegin)
		_, _, err := Anchor([]byte(src), start, end, threadsBegin, "k3f")
		if !errors.Is(err, ErrInThreads) {
			t.Errorf("got %v; want ErrInThreads", err)
		}
	})

	t.Run("nesting rules", func(t *testing.T) {
		src := "one <!--mc:a:aaa-->two three<!--mc:/a:aaa--> four"
		tests := []struct {
			name    string
			sub     string
			wantErr error
		}{
			{"fully inside an existing anchor", "two", nil},
			{"fully containing an existing anchor", "one <!--mc:a:aaa-->two three<!--mc:/a:aaa--> four", nil},
			{"disjoint", "four", nil},
			{"straddling the open marker", "one <!--mc:a:aaa-->two", ErrOverlap},
			{"straddling the close marker", "three<!--mc:/a:aaa--> four", ErrOverlap},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				start, end := offsetOf(src, tt.sub)
				_, _, err := Anchor([]byte(src), start, end, tt.sub, "bbb")
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("got %v; want %v", err, tt.wantErr)
				}
			})
		}
	})

	t.Run("expands out of a fenced code block", func(t *testing.T) {
		src := "Before\n\n" + fence + "go\nfmt.Println(1)\n" + fence + "\n\nAfter\n"
		start, end := offsetOf(src, "Println")
		got, _, err := Anchor([]byte(src), start, end, "Println", "k3f")
		if err != nil {
			t.Fatalf("Anchor: %v", err)
		}
		want := "Before\n\n<!--mc:a:k3f-->\n" + fence + "go\nfmt.Println(1)\n" + fence + "\n<!--mc:/a:k3f-->\n\nAfter\n"
		if string(got) != want {
			t.Errorf("got:\n%s\nwant:\n%s", got, want)
		}
	})

	t.Run("expands a span that only clips a fence", func(t *testing.T) {
		src := "Before\n\n" + fence + "\ncode\n" + fence + "\n\nAfter\n"
		start, _ := offsetOf(src, "Before")
		_, end := offsetOf(src, "code")
		got, _, err := Anchor([]byte(src), start, end, "", "k3f")
		if err != nil {
			t.Fatalf("Anchor: %v", err)
		}
		if !strings.Contains(string(got), fence+"\ncode\n"+fence+"\n<!--mc:/a:k3f-->") {
			t.Errorf("close marker not pushed past the fence:\n%s", got)
		}
	})
}

func TestThreads(t *testing.T) {
	t.Run("no block means no threads", func(t *testing.T) {
		got, err := Threads([]byte("just prose\n"))
		if err != nil {
			t.Fatalf("Threads: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("got %d threads; want 0", len(got))
		}
	})

	t.Run("parses threads and extension fields", func(t *testing.T) {
		src := "prose\n\n" + threadsBegin + "\n" +
			`<!--mc:t {"id":"aaa","quote":"one","status":"open","comments":[{"id":"c1","author":"olly","ts":"2026-08-02T10:00:00Z","body":"tighten this"}],"priority":"high","category":"instructions"}-->` + "\n" +
			`<!--mc:t {"id":"bbb","quote":"two","status":"resolved","comments":[]}-->` + "\n" +
			threadsEnd + "\n"

		got, err := Threads([]byte(src))
		if err != nil {
			t.Fatalf("Threads: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("got %d threads; want 2", len(got))
		}
		if got[0].ID != "aaa" || got[0].Priority != "high" || got[0].Category != "instructions" {
			t.Errorf("first thread = %+v", got[0])
		}
		if len(got[0].Comments) != 1 || got[0].Comments[0].Body != "tighten this" {
			t.Errorf("first thread comments = %+v", got[0].Comments)
		}
		if got[1].Status != "resolved" {
			t.Errorf("second thread status = %q; want resolved", got[1].Status)
		}
	})

	t.Run("malformed thread line is an error, not a silent drop", func(t *testing.T) {
		src := threadsBegin + "\n<!--mc:t {not json}-->\n" + threadsEnd + "\n"
		if _, err := Threads([]byte(src)); err == nil {
			t.Error("expected an error for a corrupt thread line")
		}
	})
}

func TestUpsert(t *testing.T) {
	thread := Thread{
		ID:       "aaa",
		Quote:    "never push",
		Status:   "open",
		Comments: []Comment{{ID: "c1", Author: "olly", TS: "2026-08-02T10:00:00Z", Body: "say why"}},
		Priority: "high",
		Category: "instructions",
	}

	t.Run("creates the block when absent", func(t *testing.T) {
		got, err := Upsert([]byte("prose\n"), thread)
		if err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		if !strings.HasPrefix(string(got), "prose\n") {
			t.Errorf("prose not preserved:\n%s", got)
		}
		round, err := Threads(got)
		if err != nil {
			t.Fatalf("Threads: %v", err)
		}
		if len(round) != 1 || round[0].ID != "aaa" || round[0].Priority != "high" {
			t.Errorf("round trip lost data: %+v", round)
		}
	})

	t.Run("replaces one line and leaves neighbours byte-identical", func(t *testing.T) {
		other := `<!--mc:t {"id":"bbb","quote":"two","status":"open","comments":[],"editedTs":"2026-01-01T00:00:00Z"}-->`
		src := "prose\n\n" + threadsBegin + "\n" +
			`<!--mc:t {"id":"aaa","quote":"old","status":"open","comments":[]}-->` + "\n" +
			other + "\n" + threadsEnd + "\n"

		got, err := Upsert([]byte(src), thread)
		if err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		if !strings.Contains(string(got), other) {
			t.Errorf("neighbouring thread line was rewritten:\n%s", got)
		}
		if strings.Contains(string(got), `"quote":"old"`) {
			t.Errorf("target thread not replaced:\n%s", got)
		}
		if n := strings.Count(string(got), `"id":"aaa"`); n != 1 {
			t.Errorf("thread aaa appears %d times; want 1", n)
		}
	})

	t.Run("appends a new thread inside the existing block", func(t *testing.T) {
		src := "prose\n\n" + threadsBegin + "\n" +
			`<!--mc:t {"id":"bbb","quote":"two","status":"open","comments":[]}-->` + "\n" +
			threadsEnd + "\n"

		got, err := Upsert([]byte(src), thread)
		if err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		round, err := Threads(got)
		if err != nil {
			t.Fatalf("Threads: %v", err)
		}
		if len(round) != 2 {
			t.Fatalf("got %d threads; want 2", len(round))
		}
		if strings.Count(string(got), threadsBegin) != 1 {
			t.Errorf("threads block duplicated:\n%s", got)
		}
	})

	t.Run("escapes a body that would otherwise close the marker", func(t *testing.T) {
		hostile := thread
		hostile.Comments[0].Body = "this --> breaks things"
		got, err := Upsert([]byte("prose\n"), hostile)
		if err != nil {
			t.Fatalf("Upsert: %v", err)
		}
		round, err := Threads(got)
		if err != nil {
			t.Fatalf("Threads: %v", err)
		}
		if round[0].Comments[0].Body != "this --> breaks things" {
			t.Errorf("body = %q", round[0].Comments[0].Body)
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("drops the thread line and the marker pair", func(t *testing.T) {
		src := "one <!--mc:a:aaa-->two<!--mc:/a:aaa--> three\n\n" + threadsBegin + "\n" +
			`<!--mc:t {"id":"aaa","quote":"two","status":"open","comments":[]}-->` + "\n" +
			`<!--mc:t {"id":"bbb","quote":"x","status":"open","comments":[]}-->` + "\n" +
			threadsEnd + "\n"

		got, err := Remove([]byte(src), "aaa")
		if err != nil {
			t.Fatalf("Remove: %v", err)
		}
		if !strings.HasPrefix(string(got), "one two three\n") {
			t.Errorf("prose not restored:\n%s", got)
		}
		round, err := Threads(got)
		if err != nil {
			t.Fatalf("Threads: %v", err)
		}
		if len(round) != 1 || round[0].ID != "bbb" {
			t.Errorf("threads after removal: %+v", round)
		}
	})

	t.Run("removes block markers without leaving blank lines", func(t *testing.T) {
		src := "Before\n\n<!--mc:a:aaa-->\n" + fence + "\ncode\n" + fence + "\n<!--mc:/a:aaa-->\n\nAfter\n" +
			"\n" + threadsBegin + "\n" +
			`<!--mc:t {"id":"aaa","quote":"code","status":"open","comments":[]}-->` + "\n" +
			threadsEnd + "\n"

		got, err := Remove([]byte(src), "aaa")
		if err != nil {
			t.Fatalf("Remove: %v", err)
		}
		want := "Before\n\n" + fence + "\ncode\n" + fence + "\n\nAfter\n"
		if !strings.HasPrefix(string(got), want) {
			t.Errorf("got:\n%s\nwant prefix:\n%s", got, want)
		}
	})

	t.Run("unknown id", func(t *testing.T) {
		_, err := Remove([]byte("prose\n"), "zzz")
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("got %v; want ErrNotFound", err)
		}
	})
}

func TestNewID(t *testing.T) {
	seen := make(map[string]bool, 1000)
	for range 1000 {
		id := NewID()
		if len(id) == 0 || len(id) > 12 {
			t.Fatalf("id %q has length %d; want 1..12", id, len(id))
		}
		for _, r := range id {
			if !isBase36(byte(r)) {
				t.Fatalf("id %q contains %q, which is not base36", id, r)
			}
		}
		if seen[id] {
			t.Fatalf("duplicate id %q within 1000 draws", id)
		}
		seen[id] = true
	}
}
