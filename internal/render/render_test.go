package render

import (
	"bytes"
	"flag"
	stdhtml "html"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "update golden files")

const fence = "```"

var spanPattern = regexp.MustCompile(`(?s)<span data-o="(\d+)">(.*?)</span>`)

// assertOffsets is the property the whole tool rests on: every span's text is
// exactly the source bytes starting at the offset it advertises.
func assertOffsets(t *testing.T, src, out []byte) {
	t.Helper()
	for _, m := range spanPattern.FindAllSubmatch(out, -1) {
		offset, err := strconv.Atoi(string(m[1]))
		if err != nil {
			t.Fatalf("unparseable offset %q", m[1])
		}
		want := stdhtml.UnescapeString(string(m[2]))
		if offset < 0 || offset+len(want) > len(src) {
			t.Errorf("span at %d claims %d bytes; source is %d bytes", offset, len(want), len(src))
			continue
		}
		if got := string(src[offset : offset+len(want)]); got != want {
			t.Errorf("span at %d renders %q; source has %q", offset, want, got)
		}
	}
}

func assertSubstrings(t *testing.T, got []byte, contains, excludes []string) {
	t.Helper()
	for _, want := range contains {
		if !strings.Contains(string(got), want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	for _, unwanted := range excludes {
		if strings.Contains(string(got), unwanted) {
			t.Errorf("unexpected %q in:\n%s", unwanted, got)
		}
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return src
}

func assertGolden(t *testing.T, got []byte, name string) {
	t.Helper()
	if *update {
		if err := os.WriteFile(filepath.Join("testdata", name), got, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	if want := readTestdata(t, name); !bytes.Equal(got, want) {
		t.Errorf("output differs from testdata/%s; re-run with -update if intended", name)
	}
}

func TestHTML(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		contains []string
		excludes []string
	}{
		{
			name:     "plain paragraph is one unbroken run",
			src:      "Hello world\n",
			contains: []string{`<p><span data-o="0">Hello world</span></p>`},
		},
		{
			name:     "emphasis splits into separate runs",
			src:      "the **lazy** fix\n",
			contains: []string{`<span data-o="0">the </span>`, `<strong><span data-o="6">lazy</span></strong>`},
		},
		{
			name:     "heading",
			src:      "# Title\n",
			contains: []string{`<h1><span data-o="2">Title</span></h1>`},
		},
		{
			name:     "code span",
			src:      "run `go test` now\n",
			contains: []string{`<code><span data-o="5">go test</span></code>`},
		},
		{
			name:     "fenced block stamps every line",
			src:      "before\n\n" + fence + "go\nfmt.Println(1)\nreturn\n" + fence + "\n",
			contains: []string{`<pre><code class="language-go">`, `<span data-o="14">fmt.Println(1)`},
		},
		{
			name:     "table cells",
			src:      "| a | b |\n| - | - |\n| c | d |\n",
			contains: []string{"<table>", `<span data-o="2">a</span>`},
		},
		{
			name:     "link text",
			src:      "see [the docs](https://example.com)\n",
			contains: []string{`<a href="https://example.com"><span data-o="5">the docs</span></a>`},
		},
		{
			name:     "html is escaped, never passed through",
			src:      "text <script>alert(1)</script> more\n",
			excludes: []string{"<script>"},
		},
		{
			name:     "inline anchor becomes a highlight",
			src:      "one <!--mc:a:k3f-->two<!--mc:/a:k3f--> three\n",
			contains: []string{`<mark class="mc" data-id="k3f">`, "</mark>"},
		},
		{
			// CommonMark would read the marker as opening an HTML block that runs to the
			// "-->", which took the whole line — prose included — out of the document.
			name: "an anchor opening a paragraph keeps the prose",
			src:  "<!--mc:a:k3f-->Compact the *whole* thing<!--mc:/a:k3f--> and more\n",
			contains: []string{
				"<p>", `<mark class="mc" data-id="k3f">`,
				`<span data-o="15">Compact the </span>`, `<em><span data-o="28">whole</span></em>`,
				`<span data-o="56"> and more</span>`,
			},
			excludes: []string{`<div class="mc"`},
		},
		{
			name:     "an anchor opening a list item keeps the prose",
			src:      "- <!--mc:a:k3f-->item<!--mc:/a:k3f-->\n",
			contains: []string{"<li>", `<mark class="mc" data-id="k3f">`, `<span data-o="17">item</span>`},
		},
		{
			name:     "a closing marker opening a line keeps the prose",
			src:      "one <!--mc:a:k3f-->two\n<!--mc:/a:k3f-->three\n",
			contains: []string{`<span data-o="39">three</span>`},
		},
		{
			name:     "block anchor wraps the whole element",
			src:      "<!--mc:a:k3f-->\n" + fence + "\ncode\n" + fence + "\n<!--mc:/a:k3f-->\n",
			contains: []string{`<div class="mc" data-id="k3f">`, "</div>"},
		},
		{
			name:     "frontmatter is a block of its own, not a rule and a heading",
			src:      "---\nname: thing\n---\n\n# Title\n",
			contains: []string{`<div class="frontmatter"><pre><span data-o="0">---`, `<h1><span data-o="23">Title</span></h1>`},
			excludes: []string{"<hr>"},
		},
		{
			name: "threads block renders as nothing",
			src: "prose\n\n<!--mc:threads:begin-->\n" +
				`<!--mc:t {"id":"k3f","quote":"two","status":"open","comments":[]}-->` +
				"\n<!--mc:threads:end-->\n",
			excludes: []string{"mc:t", "threads:begin", "quote"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTML([]byte(tt.src))
			if err != nil {
				t.Fatalf("HTML: %v", err)
			}
			assertSubstrings(t, got, tt.contains, tt.excludes)
			assertOffsets(t, []byte(tt.src), got)
		})
	}
}

func TestHTMLGolden(t *testing.T) {
	// A local copy, not the root fixture: that one is the demo document `just run`
	// opens, so reviewing it would rewrite the input this golden is pinned to.
	src := readTestdata(t, "example-SKILL.md")

	got, err := HTML(src)
	if err != nil {
		t.Fatalf("HTML: %v", err)
	}
	assertOffsets(t, src, got)
	assertGolden(t, got, "example-SKILL.html")
}

func FuzzOffsets(f *testing.F) {
	f.Add("# Title\n\ntext with **bold** and `code`\n")
	f.Add("- a\n- b\n\n> quote\n")
	f.Add("a \\* b &amp; c\n")
	f.Add(fence + "go\nx := 1\n" + fence + "\n")
	f.Add("| a | b |\n| - | - |\n| c | d |\n")
	f.Add("<!--mc:a:k3f-->text<!--mc:/a:k3f-->\n")
	f.Add("<!--mc:a:k3f-->paragraph *start*\nsecond line<!--mc:/a:k3f-->\n\nnext\n")
	f.Add("héllo wörld — em dash\n")

	f.Fuzz(func(t *testing.T, src string) {
		// CommonMark requires NUL to be rendered as U+FFFD, so one source byte
		// becomes three rendered ones and the offsets legitimately diverge. A NUL
		// cannot appear in a Markdown file anyone is reviewing.
		if strings.ContainsRune(src, 0) {
			t.Skip()
		}
		out, err := HTML([]byte(src))
		if err != nil {
			t.Fatalf("HTML: %v", err)
		}
		assertOffsets(t, []byte(src), out)
	})
}
