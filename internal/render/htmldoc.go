package render

import (
	"bufio"
	"bytes"
	"fmt"
	stdhtml "html"
	"strings"

	"golang.org/x/net/html"
)

// HTMLDoc renders an HTML source file the same way HTML renders Markdown: every run
// of source text is wrapped in a span carrying its byte offset.
//
// It tokenises rather than parses. html.Parse builds a tree that has forgotten where
// its nodes came from, whereas Tokenizer.Raw returns the source bytes of the current
// token, so accumulating len(Raw()) tracks absolute position in the source.
func HTMLDoc(src []byte) ([]byte, error) {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)

	z := html.NewTokenizer(bytes.NewReader(src))
	offset, dropping := 0, 0
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		// Raw must be measured before anything else is asked of the tokenizer: Text
		// unescapes in place and overwrites the buffer Raw points into. Slicing src
		// rather than keeping Raw's bytes sidesteps that entirely.
		start := offset
		raw := src[offset : offset+len(z.Raw())]
		offset += len(raw)

		switch tt {
		case html.StartTagToken, html.SelfClosingTagToken, html.EndTagToken:
			name, _ := z.TagName()
			tag := string(name)
			if dropSubtree[tag] {
				switch {
				case tt == html.StartTagToken:
					dropping++
				case tt == html.EndTagToken && dropping > 0:
					dropping--
				}
				continue
			}
			if dropping > 0 || !allowedElements[tag] {
				continue
			}
			writeTag(w, z, tt, tag)
		case html.TextToken:
			if dropping > 0 {
				continue
			}
			// ponytail: entities display literally; split text tokens at entity
			// boundaries if it grates.
			writeOffsetSpan(w, start, raw)
		case html.CommentToken:
			// Only mc's own markers mean anything here. A comment the author wrote is
			// invisible in the browser the file was built for, and stays invisible.
			if dropping > 0 || !bytes.HasPrefix(bytes.TrimSpace(raw), bookkeepingPrefix) {
				continue
			}
			writeMarkup(w, raw, "mark")
		}
	}

	if err := w.Flush(); err != nil {
		return nil, fmt.Errorf("render: %w", err)
	}
	return buf.Bytes(), nil
}

// The tag is rebuilt from the tokenizer rather than copied from the source, so an
// attribute that is not on the allowlist has no way of reaching the page.
func writeTag(w *bufio.Writer, z *html.Tokenizer, tt html.TokenType, tag string) {
	if tt == html.EndTagToken {
		_, _ = w.WriteString("</" + tag + ">")
		return
	}

	_, _ = w.WriteString("<" + tag)
	for {
		key, value, more := z.TagAttr()
		name := string(key)
		if allowedAttributes[name] && (!isURL[name] || safeURL(string(value))) {
			// The tokenizer has already decoded entities, so "&#106;avascript:" has
			// become "javascript:" by the time safeURL sees it.
			_, _ = w.WriteString(" " + name + `="` + stdhtml.EscapeString(string(value)) + `"`)
		}
		if !more {
			break
		}
	}
	_ = w.WriteByte('>')
}

var (
	allowedAttributes = set("href", "src", "alt", "title", "colspan", "rowspan")
	isURL             = set("href", "src")
	allowedSchemes    = set("http", "https", "mailto")
)

// A URL with no scheme is relative and safe. One with a scheme is only safe if the
// scheme is allowlisted, which rules out javascript:, data: and vbscript:.
//
// Browsers ignore ASCII whitespace and control characters inside a scheme, so
// "java\tscript:alert(1)" is a live javascript: URL; they are dropped before the
// comparison rather than trusted to fail it.
func safeURL(value string) bool {
	var scheme strings.Builder
	for _, r := range value {
		switch {
		case r == ':':
			return allowedSchemes[strings.ToLower(scheme.String())]
		case r == '/' || r == '?' || r == '#':
			return true
		case r <= 0x20 || r == 0x7f:
			continue
		default:
			scheme.WriteRune(r)
		}
	}
	return true
}

// An allowlist, not a denylist: an element nobody thought about is dropped rather than
// passed through. Anything that can execute, fetch, or collect — script, style, iframe,
// object, embed, form and every input — is simply absent from it.
var allowedElements = set(
	"p", "br", "hr", "div", "span", "pre", "code", "blockquote",
	"h1", "h2", "h3", "h4", "h5", "h6", "hgroup",
	"ul", "ol", "li", "dl", "dt", "dd",
	"table", "thead", "tbody", "tfoot", "tr", "th", "td", "caption",
	"a", "img", "figure", "figcaption",
	"em", "strong", "i", "b", "u", "s", "del", "ins", "mark", "small", "sub", "sup",
	"abbr", "cite", "dfn", "kbd", "q", "samp", "time", "var",
	"section", "article", "aside", "header", "footer", "nav", "main",
	"details", "summary",
)

// Dropping the tags of these would leave their bodies behind as visible text: a
// stylesheet or a program printed into the middle of the document. Everything between
// the tags goes with them.
//
// html, head and body are absent on purpose rather than dropped whole — the output is a
// fragment spliced into an existing page, so those tags are meaningless, but the text
// inside them is still the reviewer's to comment on.
var dropSubtree = set("script", "style")

func set(names ...string) map[string]bool {
	m := make(map[string]bool, len(names))
	for _, name := range names {
		m[name] = true
	}
	return m
}
