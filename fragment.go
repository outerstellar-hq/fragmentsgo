// Package fragmentsgo is a framework-agnostic Markdown content engine for
// Go, modeled on fragments4k: it turns a directory of Markdown files with
// YAML front matter into blog posts, static pages, and searchable content —
// with reading time, author profiles, lifecycle statuses, and SEO metadata.
//
// The core type is [Fragment]; content lives behind a [Repository] (see
// [NewFileSystemRepository]); the blog, static, search, seo, rss, sitemap,
// and httpadapter subpackages build higher-level engines on top.
package fragmentsgo

import (
	"html/template"
	"strings"
	"time"
)

// Status is a fragment's lifecycle state. Content becomes publicly visible
// only when it resolves to Published (explicitly, or via a reached
// publish-date and no expiry).
type Status string

const (
	StatusDraft     Status = "draft"
	StatusReview    Status = "review"
	StatusApproved  Status = "approved"
	StatusPublished Status = "published"
	StatusScheduled Status = "scheduled"
	StatusArchived  Status = "archived"
	StatusExpired   Status = "expired"
)

// Fragment is one parsed Markdown document.
type Fragment struct {
	// Slug is the URL identifier, from front matter or derived from Title.
	Slug string
	// Title is the document heading.
	Title string
	// Date is the publication date from front matter (zero when absent).
	Date time.Time
	// Tags and Categories come from front matter.
	Tags       []string
	Categories []string
	// Author names an author profile (see AuthorRepository).
	Author string
	// Image is a front-matter hero/cover image path.
	Image string
	// Preview is the summary text: the front-matter preview when present,
	// otherwise the first paragraph of the body.
	Preview string
	// Template names an alternate rendering template.
	Template string
	// Order positions fragments in ordered listings (projects, pages).
	Order int
	// Status is the effective lifecycle status after resolution.
	Status Status
	// URL is the computed public URL (see RepositoryOptions.URLBuilder).
	URL string

	// HTML is the sanitized rendered Markdown.
	HTML template.HTML
	// BodyText is the tag-stripped plain text of the body (search input).
	BodyText string
	// ReadingTime is the estimated reading time in minutes (200 wpm).
	ReadingTime int

	// Fields holds the raw front matter for typed access to custom keys.
	Fields map[string]any
	// SourcePath is the file the fragment was loaded from.
	SourcePath string
}

// Visible reports whether the fragment should appear publicly: published,
// publish date reached, and not expired or archived.
func (f *Fragment) Visible() bool {
	return f.Status == StatusPublished
}

// DateValue returns the date as a pointer, nil when unset — convenient for
// templates that format optional dates.
func (f *Fragment) DateValue() *time.Time {
	if f.Date.IsZero() {
		return nil
	}
	value := f.Date
	return &value
}

// GetString reads a custom front-matter field as a string.
func (f *Fragment) GetString(key string) string {
	if value, ok := f.Fields[key]; ok {
		if text, ok := value.(string); ok {
			return text
		}
	}
	return ""
}

// GetBool reads a custom front-matter field as a boolean.
func (f *Fragment) GetBool(key string) bool {
	if value, ok := f.Fields[key]; ok {
		if flag, ok := value.(bool); ok {
			return flag
		}
	}
	return false
}

// GetInt reads a custom front-matter field as an int.
func (f *Fragment) GetInt(key string) int {
	switch value := f.Fields[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	default:
		return 0
	}
}

// GetStringList reads a custom front-matter field as a string slice.
func (f *Fragment) GetStringList(key string) []string {
	switch value := f.Fields[key].(type) {
	case []string:
		return value
	case []any:
		var items []string
		for _, item := range value {
			if text, ok := item.(string); ok {
				items = append(items, text)
			}
		}
		return items
	default:
		return nil
	}
}

// HasTag reports whether the fragment carries a tag, case-insensitively.
func (f *Fragment) HasTag(tag string) bool {
	for _, candidate := range f.Tags {
		if strings.EqualFold(candidate, tag) {
			return true
		}
	}
	return false
}

// resolveStatus derives the effective status from the raw front matter: an
// explicit status wins; a future publish date schedules; a past expiry
// expires; everything else defaults to published (unreviewed content can be
// marked draft explicitly).
func resolveStatus(fields map[string]any, now time.Time) Status {
	raw := ""
	if value, ok := fields["status"].(string); ok {
		raw = strings.ToLower(strings.TrimSpace(value))
	}
	switch Status(raw) {
	case StatusDraft, StatusReview, StatusApproved, StatusArchived:
		return Status(raw)
	case StatusPublished:
		// fall through to date-based refinement
	}
	publishAt := parseTimeField(fields["publishAt"])
	if !publishAt.IsZero() && publishAt.After(now) {
		return StatusScheduled
	}
	expiresAt := parseTimeField(fields["expiresAt"])
	if !expiresAt.IsZero() && expiresAt.Before(now) {
		return StatusExpired
	}
	if publishAt.IsZero() && raw == string(StatusScheduled) {
		return StatusScheduled
	}
	return StatusPublished
}

func parseTimeField(value any) time.Time {
	switch typed := value.(type) {
	case time.Time:
		return typed
	case string:
		for _, layout := range []string{time.RFC3339, "2006-01-02", "2006-01-02 15:04"} {
			if parsed, err := time.Parse(layout, typed); err == nil {
				return parsed
			}
		}
	}
	return time.Time{}
}

// ReadingTimeOf estimates reading minutes for a plain-text body at 200
// words per minute, minimum one for non-empty text.
func ReadingTimeOf(text string) int {
	if strings.TrimSpace(text) == "" {
		return 0
	}
	minutes := (len(strings.Fields(text)) + 199) / 200
	if minutes < 1 {
		minutes = 1
	}
	return minutes
}
