// Package render turns Markdown into HTML that remembers where every character
// came from: each run of source text is wrapped in a span carrying its byte offset
// as data-o.
//
// That is the whole reason this tool renders server-side. The browser never has to
// reverse-engineer which part of the source a selection came from, it reads an
// attribute. Anchors that go wrong are the standard failure of markdown comment
// tools, and this deletes the guesswork that causes it.
package render

import (
	"bytes"
	"fmt"
	stdhtml "html"
	"regexp"
	"strconv"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	"github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// markdown is immutable once built and safe for concurrent use.
var markdown = goldmark.New(
	// GFM minus Linkify: autolinking splits text into a span per word, which bloats
	// the HTML for no gain in a document whose links are already explicit.
	goldmark.WithExtensions(extension.Table, extension.Strikethrough, extension.TaskList),
	goldmark.WithRendererOptions(
		// Lower priority registers last and so wins over goldmark's own renderer.
		renderer.WithNodeRenderers(util.Prioritized(&offsetRenderer{}, 100)),
	),
)

func HTML(src []byte) ([]byte, error) {
	var buf bytes.Buffer

	body := src
	if end := frontmatterEnd(src); end > 0 {
		fmt.Fprintf(&buf, `<div class="frontmatter"><pre><span data-o="0">%s</span></pre></div>`+"\n",
			stdhtml.EscapeString(string(src[:end])))
		// Blanking the frontmatter rather than slicing it off keeps every later
		// offset absolute, which is the whole contract with the browser.
		body = append(bytes.Repeat([]byte("\n"), end), src[end:]...)
	}

	if err := markdown.Convert(body, &buf); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return buf.Bytes(), nil
}

// Frontmatter is found so it can be handled separately: CommonMark would otherwise
// render it as a horizontal rule followed by a heading.
func frontmatterEnd(src []byte) int {
	const delimiter = "---\n"
	if !bytes.HasPrefix(src, []byte(delimiter)) {
		return 0
	}
	closing := bytes.Index(src[len(delimiter):], []byte("\n"+delimiter))
	if closing < 0 {
		return 0
	}
	return len(delimiter) + closing + len("\n") + len(delimiter)
}

type offsetRenderer struct{}

func (r *offsetRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindText, r.renderText)
	reg.Register(ast.KindCodeSpan, r.renderCodeSpan)
	reg.Register(ast.KindRawHTML, r.renderRawHTML)
	reg.Register(ast.KindHTMLBlock, r.renderHTMLBlock)
	reg.Register(ast.KindCodeBlock, r.renderCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, r.renderFencedCodeBlock)
}

func (r *offsetRenderer) renderText(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkContinue, nil
	}
	n := node.(*ast.Text)
	writeSpan(w, source, n.Segment)
	switch {
	case n.HardLineBreak():
		_, _ = w.WriteString("<br>\n")
	case n.SoftLineBreak():
		_ = w.WriteByte('\n')
	}
	return ast.WalkContinue, nil
}

// renderCodeSpan exists only because goldmark's own version writes its children's
// segments directly, which would bypass the offset stamping.
func (r *offsetRenderer) renderCodeSpan(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</code>")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<code>")
	for c := node.FirstChild(); c != nil; c = c.NextSibling() {
		if t, ok := c.(*ast.Text); ok {
			writeSpan(w, source, t.Segment)
		}
	}
	return ast.WalkSkipChildren, nil
}

func (r *offsetRenderer) renderRawHTML(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		return ast.WalkSkipChildren, nil
	}
	n := node.(*ast.RawHTML)
	for i := range n.Segments.Len() {
		seg := n.Segments.At(i)
		writeMarkup(w, seg.Value(source), "mark")
	}
	return ast.WalkSkipChildren, nil
}

func (r *offsetRenderer) renderHTMLBlock(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	n := node.(*ast.HTMLBlock)
	if !entering {
		if n.HasClosure() {
			writeMarkup(w, n.ClosureLine.Value(source), "div")
		}
		return ast.WalkContinue, nil
	}
	for i := range n.Lines().Len() {
		line := n.Lines().At(i)
		writeMarkup(w, line.Value(source), "div")
	}
	return ast.WalkContinue, nil
}

func (r *offsetRenderer) renderCodeBlock(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	_, _ = w.WriteString("<pre><code>")
	writeLines(w, source, node)
	return ast.WalkContinue, nil
}

func (r *offsetRenderer) renderFencedCodeBlock(
	w util.BufWriter, source []byte, node ast.Node, entering bool,
) (ast.WalkStatus, error) {
	if !entering {
		_, _ = w.WriteString("</code></pre>\n")
		return ast.WalkContinue, nil
	}
	n := node.(*ast.FencedCodeBlock)
	_, _ = w.WriteString("<pre><code")
	if language := n.Language(source); language != nil {
		_, _ = w.WriteString(` class="language-`)
		html.DefaultWriter.Write(w, language)
		_, _ = w.WriteString(`"`)
	}
	_ = w.WriteByte('>')
	writeLines(w, source, n)
	return ast.WalkContinue, nil
}

func writeLines(w util.BufWriter, source []byte, n ast.Node) {
	for i := range n.Lines().Len() {
		writeSpan(w, source, n.Lines().At(i))
	}
}

// A segment's padding is written outside the span so the span's text is exactly
// source[Start:Stop] — the browser's offset arithmetic depends on that being true
// byte for byte.
func writeSpan(w util.BufWriter, source []byte, seg text.Segment) {
	for range seg.Padding {
		_ = w.WriteByte(' ')
	}
	_, _ = w.WriteString(`<span data-o="`)
	_, _ = w.WriteString(strconv.Itoa(seg.Start))
	_, _ = w.WriteString(`">`)
	html.DefaultWriter.RawWrite(w, source[seg.Start:seg.Stop])
	_, _ = w.WriteString(`</span>`)
}

var (
	openMarkerPattern  = regexp.MustCompile(`^<!--mc:a:([a-z0-9]{1,12})-->$`)
	closeMarkerPattern = regexp.MustCompile(`^<!--mc:/a:[a-z0-9]{1,12}-->$`)
	bookkeepingPrefix  = []byte("<!--mc:")
)

// Raw HTML that is not ours is escaped and shown, never executed.
func writeMarkup(w util.BufWriter, raw []byte, element string) {
	markup := bytes.TrimSpace(raw)
	switch {
	case openMarkerPattern.Match(markup):
		id := openMarkerPattern.FindSubmatch(markup)[1]
		_, _ = w.WriteString("<" + element + ` class="mc" data-id="`)
		_, _ = w.Write(id)
		_, _ = w.WriteString(`">`)
	case closeMarkerPattern.Match(markup):
		_, _ = w.WriteString("</" + element + ">")
	case bytes.HasPrefix(markup, bookkeepingPrefix):
		// The threads block and anything else mc owns renders as nothing.
	default:
		html.DefaultWriter.RawWrite(w, raw)
	}
}
