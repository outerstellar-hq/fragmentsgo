package fragmentsgo

import (
	"bytes"
	"regexp"
	"strings"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
)

// SanitizerProfile selects the HTML policy applied to rendered Markdown.
type SanitizerProfile int

const (
	// SanitizerRelaxedTrustedAuthor allows standard rich content (images,
	// links, headings, code, tables) — the default, matching fragments4k's
	// RELAXED_TRUSTED_AUTHOR profile for content you author yourself.
	SanitizerRelaxedTrustedAuthor SanitizerProfile = iota
	// SanitizerStrict strips everything down to basic formatting — for
	// content contributed by untrusted parties.
	SanitizerStrict
)

// MarkdownParser converts Markdown bodies into sanitized HTML.
type MarkdownParser struct {
	markdown goldmark.Markdown
	sanitize *bluemonday.Policy
}

// NewMarkdownParser builds a parser with the given sanitizer profile.
func NewMarkdownParser(profile SanitizerProfile) *MarkdownParser {
	var policy *bluemonday.Policy
	switch profile {
	case SanitizerStrict:
		strict := bluemonday.StrictPolicy()
		strict.AllowStandardURLs()
		policy = strict
	default:
		ugc := bluemonday.UGCPolicy()
		ugc.AllowAttrs("class").Matching(bluemonday.SpaceSeparatedTokens).OnElements("code", "span", "div", "a")
		ugc.AllowAttrs("id").Matching(regexp.MustCompile(`^[\w-]+$`)).OnElements("h1", "h2", "h3", "h4", "h5", "h6")
		ugc.AllowImages()
		policy = ugc
	}
	return &MarkdownParser{
		markdown: goldmark.New(
			goldmark.WithExtensions(
				extension.GFM,
				extension.Footnote,
				extension.Linkify,
				extension.TaskList,
			),
			goldmark.WithParserOptions(parser.WithAutoHeadingID()),
			goldmark.WithRendererOptions(html.WithUnsafe()),
		),
		sanitize: policy,
	}
}

// Render converts Markdown to sanitized HTML.
func (p *MarkdownParser) Render(source []byte) (string, error) {
	var rendered bytes.Buffer
	if err := p.markdown.Convert(source, &rendered); err != nil {
		return "", err
	}
	return p.sanitize.Sanitize(rendered.String()), nil
}

var (
	tagStripper = regexp.MustCompile(`<[^>]*>`)
	whitespace  = regexp.MustCompile(`\s+`)
)

// PlainText strips HTML tags and collapses whitespace.
func PlainText(html string) string {
	text := tagStripper.ReplaceAllString(html, " ")
	return strings.TrimSpace(whitespace.ReplaceAllString(text, " "))
}

// FirstParagraph extracts the first prose paragraph of a Markdown body as
// preview text: headings, lists, code fences, tables, and quotes are
// skipped.
func FirstParagraph(body string) string {
	for _, line := range strings.Split(body, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") || strings.HasPrefix(line, "-") ||
			strings.HasPrefix(line, "*") || strings.HasPrefix(line, "```") ||
			strings.HasPrefix(line, "|") || strings.HasPrefix(line, ">") ||
			strings.HasPrefix(line, "[") {
			continue
		}
		return line
	}
	return ""
}

// Slugify converts a title into a URL slug.
func Slugify(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var builder strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			builder.WriteRune(r)
			lastDash = false
		} else if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}
