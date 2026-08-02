package render

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func TestHTMLDoc(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		contains []string
		excludes []string
	}{
		{
			name:     "text keeps its element and carries its offset",
			src:      "<p>Hello world</p>\n",
			contains: []string{`<p><span data-o="3">Hello world</span></p>`},
		},
		{
			name:     "script is dropped along with its body",
			src:      "<p>before</p><script>alert(1)</script><p>after</p>\n",
			contains: []string{`<span data-o="3">before</span>`, `<span data-o="41">after</span>`},
			excludes: []string{"script", "alert(1)"},
		},
		{
			// The tokenizer hands these back as one unparsed text token, so dropping only
			// the tags would print raw markup into the page as visible text.
			name: "rawtext elements lose their bodies too",
			src: "<iframe src=\"/x\"><script>alert(1)</script></iframe>" +
				"<noscript><b>no js</b></noscript><xmp><i>x</i></xmp><p>kept</p>\n",
			contains: []string{`<span data-o="106">kept</span>`},
			excludes: []string{"iframe", "alert(1)", "no js", "noscript", "&lt;"},
		},
		{
			// Their children are parsed markup, not a raw slab: with the tag gone and no
			// allowlisted attribute left there is nothing to embed or submit.
			name:     "object and form lose their tags but keep their words",
			src:      `<object data="/x"><b>fallback</b></object><form action="/s"><label>Email</label></form>` + "\n",
			contains: []string{"<b><span data-o=", "fallback", "Email"},
			excludes: []string{"object", "<form", "action", "data="},
		},
		{
			name:     "only allowlisted attributes survive",
			src:      `<a href="/docs" title="t" onclick="evil()" class="x" data-o="999">link</a>` + "\n",
			contains: []string{`<a href="/docs" title="t">`},
			excludes: []string{"onclick", "evil", `class=`, `data-o="999"`},
		},
		{
			name:     "on* attributes and srcdoc are stripped from every element",
			src:      `<div onmouseover="evil()"><img src="/a.png" alt="a" onerror="evil()" srcdoc="<script>"></div>` + "\n",
			contains: []string{`<img src="/a.png" alt="a">`},
			excludes: []string{"onmouseover", "onerror", "srcdoc", "evil"},
		},
		{
			name: "http, https, mailto and relative URLs are kept",
			src: `<a href="https://example.com">a</a><a href="http://example.com">b</a>` +
				`<a href="mailto:x@example.com">c</a><a href="./rel#frag">d</a>` + "\n",
			contains: []string{
				`href="https://example.com"`, `href="http://example.com"`,
				`href="mailto:x@example.com"`, `href="./rel#frag"`,
			},
		},
		{
			name: "every other scheme is dropped, however it is spelled",
			src: `<a href="javascript:evil()">a</a>` +
				`<a href="  JaVaScRiPt:evil()">b</a>` +
				"<a href=\"java\tscript:evil()\">c</a>" +
				`<a href="&#106;avascript:evil()">d</a>` +
				`<a href="data:text/html,&lt;script&gt;">e</a>` +
				`<a href="vbscript:evil()">f</a>` + "\n",
			contains: []string{`<a>`},
			excludes: []string{"href=", "evil", "vbscript", "data:"},
		},
		{
			name:     "anchor markers become a highlight",
			src:      "<p>one <!--mc:a:k3f-->two<!--mc:/a:k3f--> three</p>\n",
			contains: []string{`<mark class="mc" data-id="k3f"><span data-o="22">two</span></mark>`},
			excludes: []string{"mc:a:k3f"},
		},
		{
			name: "the threads block renders as nothing",
			src: "<p>prose</p>\n</html>\n\n<!--mc:threads:begin-->\n" +
				`<!--mc:t {"id":"k3f","quote":"two","status":"open","comments":[]}-->` +
				"\n<!--mc:threads:end-->\n",
			excludes: []string{"mc:t", "threads:begin", "quote"},
		},
		{
			name:     "a comment that is not ours is invisible, as it is in a browser",
			src:      "<p>one<!-- reviewer note -->two</p>\n",
			excludes: []string{"reviewer note", "<!--"},
		},
		{
			// The offset property wins over pretty output: a source entity is escaped
			// again so that unescaping the span recovers the source bytes exactly.
			name:     "entities stay as source bytes so offsets keep counting",
			src:      "<p>a &lt; b &nbsp; c</p>\n",
			contains: []string{`<span data-o="3">a &amp;lt; b &amp;nbsp; c</span>`},
		},
		{
			// The browser's parser drops the CR, so a span holding one would advertise
			// a byte the browser cannot count and every later offset in it would drift.
			name:     "a CR ends the run instead of sitting inside it",
			src:      "<p>a\r\nb</p>\n",
			contains: []string{`<span data-o="3">a</span>` + "\r" + `<span data-o="5">` + "\nb</span>"},
		},
		{
			name:     "pre content is one run, whitespace and all",
			src:      "<pre>\n  one\n  two\n</pre>\n",
			contains: []string{"<pre><span data-o=\"5\">\n  one\n  two\n</span></pre>"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := HTMLDoc([]byte(tt.src))
			if err != nil {
				t.Fatalf("HTMLDoc: %v", err)
			}
			for _, want := range tt.contains {
				if !strings.Contains(string(got), want) {
					t.Errorf("missing %q in:\n%s", want, got)
				}
			}
			for _, unwanted := range tt.excludes {
				if strings.Contains(string(got), unwanted) {
					t.Errorf("unexpected %q in:\n%s", unwanted, got)
				}
			}
			assertOffsets(t, []byte(tt.src), got)
		})
	}
}

func TestHTMLDocGolden(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "example-report.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	got, err := HTMLDoc(src)
	if err != nil {
		t.Fatalf("HTMLDoc: %v", err)
	}
	assertOffsets(t, src, got)

	golden := filepath.Join("testdata", "example-report.rendered.html")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("update golden: %v", err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output differs from %s; re-run with -update if intended", golden)
	}
}

func FuzzHTMLOffsets(f *testing.F) {
	f.Add("<p>Hello <strong>world</strong></p>\n")
	f.Add("<p>windows\r\nline\r\nendings</p>\r\n")
	f.Add("<p>a &lt; b &amp; c &nbsp; d</p>\n")
	f.Add("<pre>\n  indented\n</pre>\n")
	f.Add("<!doctype html><html><head><title>t</title></head><body>x</body></html>\n")
	f.Add(`<a href="javascript:evil()" onclick="evil()">link</a>` + "\n")
	f.Add("<p>one<!--mc:a:k3f-->two<!--mc:/a:k3f-->three</p>\n")
	f.Add("<table><tr><td colspan=\"2\">héllo — wörld</td></tr></table>\n")
	f.Add("<div><script>alert(1)</script><style>b{}</style></div>\n")
	f.Add("<p unclosed attr='<<<'\n")

	f.Fuzz(func(t *testing.T, src string) {
		// The HTML spec makes NUL a U+FFFD, and so does the writer, so one source byte
		// becomes three rendered ones and the offsets legitimately diverge — the same
		// exemption FuzzOffsets makes, for the same reason.
		if strings.ContainsRune(src, 0) {
			t.Skip()
		}
		out, err := HTMLDoc([]byte(src))
		if err != nil {
			t.Fatalf("HTMLDoc: %v", err)
		}
		assertOffsets(t, []byte(src), out)
	})
}

// assertOffsets reads the output as bytes. The browser reads it as a DOM, and the two
// disagree: the parser resolves entities and folds CRLF to LF before textContent exists,
// which is what app.js measures. This is that check — every span's textContent must be
// exactly the source bytes at the offset it advertises.
func assertDOMOffsets(t *testing.T, src, out []byte) {
	t.Helper()

	root, err := html.Parse(bytes.NewReader(out))
	if err != nil {
		t.Fatalf("parse output: %v", err)
	}

	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
		if n.Type != html.ElementNode || n.Data != "span" {
			return
		}
		for _, attr := range n.Attr {
			if attr.Key != "data-o" {
				continue
			}
			offset, err := strconv.Atoi(attr.Val)
			if err != nil {
				t.Fatalf("unparseable offset %q", attr.Val)
			}
			var text strings.Builder
			for child := n.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					text.WriteString(child.Data)
				}
			}
			want := text.String()
			if offset+len(want) > len(src) {
				t.Errorf("span at %d claims %d bytes; source is %d", offset, len(want), len(src))
				return
			}
			if got := string(src[offset : offset+len(want)]); got != want {
				t.Errorf("span at %d reads %q in the DOM; source has %q", offset, want, got)
			}
		}
	}
	walk(root)
}

func TestHTMLDocAnchorsInTheDOM(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "example-report.html"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	sources := map[string][]byte{
		"golden fixture": fixture,
		"crlf":           []byte("<p>windows\r\nline\r\nendings</p>\r\n<pre>a\r\nb</pre>\r\n"),
		"entities":       []byte("<p>a &lt; b &amp; c &nbsp; d</p>\n"),
		"anchored":       []byte("<p>one <!--mc:a:k3f-->two<!--mc:/a:k3f--> three</p>\n"),
	}
	for name, src := range sources {
		t.Run(name, func(t *testing.T) {
			out, err := HTMLDoc(src)
			if err != nil {
				t.Fatalf("HTMLDoc: %v", err)
			}
			assertDOMOffsets(t, src, out)
		})
	}
}
