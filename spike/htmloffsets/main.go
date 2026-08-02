// Spike for issue #7. Throwaway — delete once #10 is planned.
//
// The claim under test: html.Tokenizer.Raw() returns the source bytes of the
// current token, so accumulating len(Raw()) across Next() tracks absolute byte
// position in the source.
//
//	go run ./spike/htmloffsets     # exit 0 confirms, 1 refutes
//
// CONFIRMED against golang.org/x/net v0.57.0. src[offset:offset+len(Raw())]
// equals Raw() at every token and the total consumes the source exactly, over
// a realistic document and 28 adversarial inputs. html.Parse is confirmed
// unusable: html.Node carries no offset. #10's emission plan — RawWrite the
// source bytes, never Text() — stands as written.
//
// Three things #10 should absorb:
//
//  1. Text() unescapes entities and normalises newlines *in place*, so Raw()
//     read afterwards is corrupt, and Token() calls Text() internally. The
//     rule for htmldoc.go: read Raw(), never call Text() or Token().
//  2. FuzzHTMLOffsets needs the same NUL skip FuzzOffsets has. Not for
//     goldmark's reason (CommonMark mandating U+FFFD) but because
//     util.htmlEscapeTable[0] is U+FFFD, so RawWrite turns one source byte
//     into three.
//  3. The one genuinely new risk: assertOffsets is not the browser. It
//     compares UnescapeString(span), but app.js:69 measures
//     byteLength(span.textContent), and an HTML parser folds CR and CRLF to
//     LF and NUL to U+FFFD before textContent exists. A source CRLF inside a
//     text token therefore passes the Go test and drifts a byte per line in
//     the page. Markdown never hit this because goldmark leaves CR outside
//     its segments; the HTML tokenizer keeps it inside Raw(). No emission
//     strategy can fix it — #10 should normalise CRLF at read time, or
//     refuse to anchor across a CR. The "browser textContent" column below is
//     the one that binds, not the assertOffsets column.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	stdhtml "html"
	"os"
	"strings"

	gmhtml "github.com/yuin/goldmark/renderer/html"
	"golang.org/x/net/html"
)

const sample = `<!DOCTYPE html>
<html lang="en">
<head><meta charset="utf-8"><title>A &amp; B</title></head>
<body>
  <!-- a comment, and an mc marker below -->
  <!--mc:a:k3f-->
  <div class="wrap" data-n='3'>
    <p>text with &lt;angles&gt; and &nbsp; a gap</p>
    <ul><li>one</li><li>two</li></ul>
  </div>
  <!--mc:/a:k3f-->
  <pre>  indented
	tabbed &lt;not a tag&gt;
</pre>
  <img src="x.png" alt="an &quot;image&quot;">
  <p>trailing &#233; and &eacute;</p>
</body>
</html>
`

// adversarial covers the shapes a realistic sample will not: broken markup,
// rawtext elements, CRLF, and the things a sanitiser has to survive.
var adversarial = []string{
	"",
	"plain text, no tags at all",
	"<p>unclosed",
	"</p>stray close",
	"<a href=unquoted class=x>y</a>",
	"a < b and 3 > 2",
	"<script>if (a<b) { x() }</script>",
	"<style>p::before{content:'<'}</style>",
	"<textarea><p>not a tag</p></textarea>",
	"<title>&amp; in rawtext</title>",
	"line one\r\nline two\r\n",
	"<p>\r\n\rmixed\r</p>",
	"<!-- unterminated comment",
	"<![CDATA[not really]]>",
	"<p>&notanentity; &amp &#x41;</p>",
	"<p>héllo wörld — em dash 🙂</p>",
	"<p>nul\x00byte</p>",
	"<plaintext>everything after is text <p>",
	"<div<div>",
	"<p a=\"1\" a=\"2\" >dup attrs</p>",
	"<!--mc:a:k3f-->x<!--mc:/a:k3f-->",
	"<svg><foreignObject><p>mixed</p></foreignObject></svg>",
	"<b>bold<i>both</b>italic</i>",
	"<?processing instruction?>",
	"<!DOCTYPE html SYSTEM \"about:legacy-compat\">",
	"\ufeff<p>leading BOM</p>",
	"<pre>\nleading newline the parser would drop</pre>",
	"<pre>&lt;tag&gt;\n\tkeeps whitespace  </pre>",
}

func main() {
	fails := 0

	fmt.Println("== realistic sample: offset -> source slice ==")
	fails += walk(sample, true)

	fmt.Println("\n== adversarial inputs ==")
	adversarialFails := 0
	for _, src := range adversarial {
		if n := walk(src, false); n > 0 {
			fmt.Printf("  FAIL on %q\n", src)
			adversarialFails += n
		}
	}
	fmt.Printf("  %d inputs, %d failures\n", len(adversarial), adversarialFails)
	fails += adversarialFails

	fmt.Println("\n== Text() vs Raw() on entities ==")
	textVsRaw("<p>&lt;angles&gt; &nbsp; &#233;</p>")
	fmt.Println("\n== CRLF: Text() normalises, Raw() does not ==")
	textVsRaw("<p>a\r\nb</p>")

	fmt.Println("\n== html.Parse: does the tree carry offsets? ==")
	parseOffsets()

	fmt.Println("\n== buffer reuse: Raw() after Text() ==")
	rawAfterText("<p>&lt;x&gt;</p>")

	fmt.Println("\n== emission strategies, checked two ways ==")
	fmt.Printf("  %-26s %-22s %-22s %s\n", "strategy", "assertOffsets", "browser textContent", "input")
	for _, in := range []emissionInput{
		{src: "<p>&lt;angles&gt;</p>"},
		{src: "<p>a &amp; b</p>"},
		{src: "<p>&nbsp;gap</p>"},
		{src: "<p>plain text</p>"},
		{src: sample},
		// The finding: the HTML parser folds CR into LF and NUL into U+FFFD
		// before textContent exists, so these cannot anchor whatever we emit.
		{src: "<p>crlf\r\nsplit</p>", browserMustDrift: true},
		{src: "<p>nul\x00byte</p>", browserMustDrift: true},
	} {
		fails += emissionStrategies(in)
	}

	if fails > 0 {
		fmt.Printf("\nRESULT: REFUTED — %d mismatches\n", fails)
		os.Exit(1)
	}
	fmt.Println("\nRESULT: CONFIRMED — accumulated len(Raw()) tracked absolute source position on every input")
}

// walk is the assertion: at every token the running offset must point at the
// token's own raw bytes, and the total must consume the source exactly.
func walk(src string, verbose bool) int {
	fails := 0
	z := html.NewTokenizer(strings.NewReader(src))
	offset := 0

	for {
		tt := z.Next()
		raw := string(z.Raw())

		if tt == html.ErrorToken {
			if len(raw) != 0 {
				fmt.Printf("  MISMATCH: ErrorToken carries %d raw bytes %q\n", len(raw), raw)
				fails++
			}
			break
		}

		switch {
		case offset+len(raw) > len(src):
			fmt.Printf("  MISMATCH: %v at %d claims %d bytes; source is %d\n", tt, offset, len(raw), len(src))
			fails++
		case src[offset:offset+len(raw)] != raw:
			fmt.Printf("  MISMATCH: %v at %d\n    raw:    %q\n    source: %q\n",
				tt, offset, raw, src[offset:offset+len(raw)])
			fails++
		case verbose && (tt == html.TextToken || tt == html.CommentToken):
			fmt.Printf("  %5d  %-12v %q\n", offset, tt, src[offset:offset+len(raw)])
		}
		offset += len(raw)
	}

	if fails == 0 && offset != len(src) {
		fmt.Printf("  MISMATCH: consumed %d of %d bytes (%q dropped)\n", offset, len(src), src[min(offset, len(src)):])
		fails++
	}
	return fails
}

func textVsRaw(src string) {
	z := html.NewTokenizer(strings.NewReader(src))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return
		}
		if tt != html.TextToken {
			continue
		}
		// Raw() must be read before Text(): Text() mutates the shared buffer.
		raw := string(z.Raw())
		text := string(z.Text())
		fmt.Printf("  raw  %2d bytes %q\n  text %2d bytes %q\n", len(raw), raw, len(text), text)
	}
}

// rawAfterText pins the ordering gotcha: Text() decodes in place, so Raw()
// read afterwards no longer holds the source bytes.
func rawAfterText(src string) {
	z := html.NewTokenizer(strings.NewReader(src))
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return
		}
		if tt != html.TextToken {
			continue
		}
		text := string(z.Text())
		fmt.Printf("  Text() %q then Raw() %q  (source token was %q)\n", text, z.Raw(), "&lt;x&gt;")
	}
}

type strategy struct {
	name string
	emit func(raw, text string) string
	// mustPass under the browser model; the Go-side assertOffsets model is
	// reported for contrast but is not what binds at runtime.
	mustPass bool
}

// rawWrite is what #10 means by "RawWrite the source bytes": goldmark's writer,
// already used by render.go:194. It is "raw" only in that it does not resolve
// character references — it still escapes & < > ".
func rawWrite(s string) string {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	gmhtml.DefaultWriter.RawWrite(w, []byte(s))
	_ = w.Flush()
	return buf.String()
}

type emissionInput struct {
	src string
	// browserMustDrift inverts the check: the divergence is itself the finding,
	// so a run where it stopped happening should fail and be re-examined.
	browserMustDrift bool
}

// emissionStrategies checks each candidate two ways: against render_test.go's
// assertOffsets, and against what the browser actually measures.
func emissionStrategies(in emissionInput) int {
	strategies := []strategy{
		{name: "RawWrite(Raw()) [#10]", emit: func(raw, _ string) string { return rawWrite(raw) }, mustPass: true},
		{name: "EscapeString(Raw())", emit: func(raw, _ string) string { return stdhtml.EscapeString(raw) }, mustPass: true},
		{name: "RawWrite(Text())", emit: func(_, text string) string { return rawWrite(text) }},
	}

	fails := 0
	for _, s := range strategies {
		testBad, browserBad := 0, 0
		z := html.NewTokenizer(strings.NewReader(in.src))
		offset := 0
		for {
			tt := z.Next()
			raw := string(z.Raw())
			if tt == html.ErrorToken {
				break
			}
			if tt == html.TextToken {
				emitted := s.emit(raw, string(z.Text()))
				if !matches(in.src, offset, stdhtml.UnescapeString(emitted)) {
					testBad++
				}
				if !matches(in.src, offset, textContent(emitted)) {
					browserBad++
				}
			}
			offset += len(raw)
		}
		fmt.Printf("  %-26s %-22s %-22s %.30q\n", s.name, verdict(testBad), verdict(browserBad), in.src)
		if !s.mustPass {
			continue
		}
		if in.browserMustDrift && browserBad == 0 {
			fmt.Printf("    UNEXPECTED: %q no longer drifts in the browser; re-check the finding\n", in.src)
			fails++
		}
		if !in.browserMustDrift && browserBad > 0 {
			fails++
		}
	}
	return fails
}

func matches(src string, offset int, want string) bool {
	return offset+len(want) <= len(src) && src[offset:offset+len(want)] == want
}

func verdict(bad int) string {
	if bad == 0 {
		return "pass"
	}
	return fmt.Sprintf("FAIL (%d spans)", bad)
}

// textContent parses emitted markup the way a browser does and returns what
// span.textContent would hold — the string app.js measures with TextEncoder.
func textContent(emitted string) string {
	doc, err := html.Parse(strings.NewReader("<span>" + emitted + "</span>"))
	if err != nil {
		return ""
	}
	var b strings.Builder
	var collect func(*html.Node)
	collect = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			collect(c)
		}
	}
	collect(doc)
	return b.String()
}

func parseOffsets() {
	doc, err := html.Parse(strings.NewReader("<p>hello</p>"))
	if err != nil {
		fmt.Println("  parse:", err)
		return
	}
	var walkNode func(*html.Node)
	walkNode = func(n *html.Node) {
		if n.Type == html.TextNode {
			fmt.Printf("  TextNode %q — node fields: Type, DataAtom, Data, Namespace, Attr. No offset.\n", n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walkNode(c)
		}
	}
	walkNode(doc)
}
