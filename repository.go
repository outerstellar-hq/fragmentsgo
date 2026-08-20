package fragmentsgo

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// ErrNotFound is returned when no fragment matches a lookup.
var ErrNotFound = errors.New("fragmentsgo: fragment not found")

// RepositoryOptions configures a [FileSystemRepository].
type RepositoryOptions struct {
	// Path is the content directory holding .md files.
	Path string
	// BaseURL prefixes generated URLs (e.g. "/projects"). Mutually
	// exclusive with URLBuilder.
	BaseURL string
	// URLBuilder computes a fragment's public URL; when nil the URL is
	// BaseURL + "/" + slug (or "/"+slug without a base).
	URLBuilder func(f *Fragment) string
	// Parser renders Markdown; nil selects the relaxed trusted-author
	// profile.
	Parser *MarkdownParser
	// Now overrides time for status resolution (tests).
	Now func() time.Time
	// IncludeInvisible keeps drafts and scheduled fragments in listings so
	// preview surfaces can show them; they are excluded by default.
	IncludeInvisible bool
	// Ordered selects listing order: true sorts by Order then Title
	// (pages, projects); false (default) sorts dated content newest first.
	Ordered bool
	// Exclude drops files whose base name matches (checked case-blind
	// against the full path), e.g. to keep self-referential entries out.
	Exclude func(path string) bool
}

// Repository provides access to a set of fragments.
type Repository interface {
	// Load (re)reads every fragment from disk.
	Load() error
	// All returns fragments in listing order (newest first for dated
	// content; Order then Title for undated content), visible ones only
	// unless IncludeInvisible was set.
	All() []*Fragment
	// BySlug finds a fragment by slug.
	BySlug(slug string) (*Fragment, error)
	// ByURL finds a fragment by its public URL.
	ByURL(url string) (*Fragment, error)
	// UpdateStatus rewrites a fragment's status front matter and reloads it.
	UpdateStatus(slug string, status Status) error
	// Schedule sets a future publish date and reloads the fragment.
	Schedule(slug string, at time.Time) error
	// Archive marks a fragment archived and reloads it.
	Archive(slug string) error
}

type fileSystemRepository struct {
	options   RepositoryOptions
	parser    *MarkdownParser
	now       func() time.Time
	mu        sync.RWMutex
	fragments []*Fragment
	bySlug    map[string]*Fragment
	byURL     map[string]*Fragment
	// bySlugAll indexes every parsed fragment, visible or not, so
	// lifecycle mutations can find drafts and scheduled content.
	bySlugAll map[string]*Fragment
}

// NewFileSystemRepository creates a repository over a directory of Markdown
// files. Call Load before first use.
func NewFileSystemRepository(options RepositoryOptions) Repository {
	parser := options.Parser
	if parser == nil {
		parser = NewMarkdownParser(SanitizerRelaxedTrustedAuthor)
	}
	now := options.Now
	if now == nil {
		now = time.Now
	}
	return &fileSystemRepository{
		options: options,
		parser:  parser,
		now:     now,
		bySlug:  map[string]*Fragment{},
		byURL:   map[string]*Fragment{},
	}
}

// Load reads and parses every .md file in the content directory. A missing
// directory yields an empty repository so optional sections can be omitted.
func (r *fileSystemRepository) Load() error {
	entries, err := os.ReadDir(r.options.Path)
	if os.IsNotExist(err) {
		entries = nil
	} else if err != nil {
		return fmt.Errorf("fragmentsgo: read content dir: %w", err)
	}
	now := r.now()
	fragments := make([]*Fragment, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".md") {
			continue
		}
		if r.options.Exclude != nil && r.options.Exclude(entry.Name()) {
			continue
		}
		fragment, err := r.parseFile(filepath.Join(r.options.Path, entry.Name()), now)
		if err != nil {
			return err
		}
		fragments = append(fragments, fragment)
	}
	bySlug := map[string]*Fragment{}
	byURL := map[string]*Fragment{}
	bySlugAll := map[string]*Fragment{}
	var visible []*Fragment
	for _, fragment := range fragments {
		bySlugAll[fragment.Slug] = fragment
		if !fragment.Visible() && !r.options.IncludeInvisible {
			continue
		}
		visible = append(visible, fragment)
		bySlug[fragment.Slug] = fragment
		byURL[fragment.URL] = fragment
	}
	r.mu.Lock()
	r.fragments = visible
	if r.options.Ordered {
		SortOrdered(visible)
	} else {
		SortDated(visible)
	}
	r.bySlug = bySlug
	r.byURL = byURL
	r.bySlugAll = bySlugAll
	r.mu.Unlock()
	return nil
}

func (r *fileSystemRepository) parseFile(path string, now time.Time) (*Fragment, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("fragmentsgo: read %s: %w", path, err)
	}
	fields, body, err := splitFrontMatter(data)
	if err != nil {
		return nil, fmt.Errorf("fragmentsgo: parse %s: %w", path, err)
	}
	rendered, err := r.parser.Render(body)
	if err != nil {
		return nil, fmt.Errorf("fragmentsgo: render %s: %w", path, err)
	}
	fragment := &Fragment{
		Fields:     fields,
		Template:   stringField(fields, "template"),
		Author:     stringField(fields, "author"),
		Image:      stringField(fields, "image"),
		SourcePath: path,
	}
	fragment.Title = stringField(fields, "title")
	if fragment.Title == "" {
		fragment.Title = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	fragment.Slug = stringField(fields, "slug")
	if fragment.Slug == "" {
		fragment.Slug = Slugify(fragment.Title)
	}
	fragment.Date = parseTimeField(fields["date"])
	fragment.Updated = parseTimeField(fields["updated"])
	fragment.Tags = stringListField(fields, "tags")
	fragment.Categories = stringListField(fields, "categories")
	fragment.Preview = stringField(fields, "preview")
	if fragment.Preview == "" {
		fragment.Preview = FirstParagraph(string(body))
	}
	fragment.Order = intField(fields, "order")
	fragment.Status = resolveStatus(fields, now)
	fragment.HTML = template.HTML(rendered)
	fragment.BodyText = PlainText(rendered)
	fragment.ReadingTime = ReadingTimeOf(fragment.BodyText)
	fragment.URL = r.urlFor(fragment)
	return fragment, nil
}

func (r *fileSystemRepository) urlFor(fragment *Fragment) string {
	if r.options.URLBuilder != nil {
		return r.options.URLBuilder(fragment)
	}
	if r.options.BaseURL != "" {
		return strings.TrimRight(r.options.BaseURL, "/") + "/" + fragment.Slug
	}
	return "/" + fragment.Slug
}

func (r *fileSystemRepository) All() []*Fragment {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Fragment, len(r.fragments))
	copy(out, r.fragments)
	return out
}

func (r *fileSystemRepository) BySlug(slug string) (*Fragment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if fragment, ok := r.bySlug[slug]; ok {
		return fragment, nil
	}
	return nil, ErrNotFound
}

func (r *fileSystemRepository) ByURL(url string) (*Fragment, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if fragment, ok := r.byURL[url]; ok {
		return fragment, nil
	}
	return nil, ErrNotFound
}

// UpdateStatus rewrites the status in the fragment's front matter and
// reloads the file.
func (r *fileSystemRepository) UpdateStatus(slug string, status Status) error {
	return r.rewriteFrontMatter(slug, func(fields map[string]any) {
		fields["status"] = string(status)
	})
}

// Schedule sets publishAt to a future time (status scheduled).
func (r *fileSystemRepository) Schedule(slug string, at time.Time) error {
	return r.rewriteFrontMatter(slug, func(fields map[string]any) {
		fields["publishAt"] = at.Format(time.RFC3339)
		delete(fields, "status")
	})
}

// Archive marks the fragment archived.
func (r *fileSystemRepository) Archive(slug string) error {
	return r.UpdateStatus(slug, StatusArchived)
}

func (r *fileSystemRepository) rewriteFrontMatter(slug string, mutate func(map[string]any)) error {
	r.mu.RLock()
	fragment, ok := r.bySlugAll[slug]
	path := ""
	if ok {
		path = fragment.SourcePath
	}
	r.mu.RUnlock()
	if !ok {
		return ErrNotFound
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("fragmentsgo: read %s: %w", path, err)
	}
	fields, body, err := splitFrontMatter(data)
	if err != nil {
		return err
	}
	mutate(fields)
	var front bytes.Buffer
	encoder := yaml.NewEncoder(&front)
	encoder.SetIndent(2)
	if err := encoder.Encode(fields); err != nil {
		return err
	}
	if err := encoder.Close(); err != nil {
		return err
	}
	output := append([]byte("---\n"+front.String()+"---\n\n"), body...)
	if err := os.WriteFile(path, output, 0o644); err != nil {
		return fmt.Errorf("fragmentsgo: write %s: %w", path, err)
	}
	return r.Load()
}

// SortDated orders dated fragments newest-first, title as tiebreaker.
func SortDated(fragments []*Fragment) {
	sort.SliceStable(fragments, func(i, j int) bool {
		if fragments[i].Date.Equal(fragments[j].Date) {
			return fragments[i].Title < fragments[j].Title
		}
		return fragments[i].Date.After(fragments[j].Date)
	})
}

// SortOrdered orders fragments by Order, then Title (pages, projects).
func SortOrdered(fragments []*Fragment) {
	sort.SliceStable(fragments, func(i, j int) bool {
		if fragments[i].Order != fragments[j].Order {
			return fragments[i].Order < fragments[j].Order
		}
		return fragments[i].Title < fragments[j].Title
	})
}

func splitFrontMatter(data []byte) (map[string]any, []byte, error) {
	text := string(data)
	if !strings.HasPrefix(text, "---") {
		return map[string]any{}, data, nil
	}
	start := strings.IndexByte(text, '\n')
	if start < 0 {
		return nil, nil, errors.New("front matter opener is not terminated")
	}
	rest := text[start+1:]
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return nil, nil, errors.New("front matter closing marker is missing")
	}
	front := rest[:end]
	body := rest[end+len("\n---"):]
	body = strings.TrimPrefix(body, "\n")
	var fields map[string]any
	if err := yaml.Unmarshal([]byte(front), &fields); err != nil {
		return nil, nil, err
	}
	if fields == nil {
		fields = map[string]any{}
	}
	return fields, []byte(body), nil
}

func stringField(fields map[string]any, key string) string {
	if value, ok := fields[key].(string); ok {
		return value
	}
	return ""
}

func intField(fields map[string]any, key string) int {
	switch value := fields[key].(type) {
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

func stringListField(fields map[string]any, key string) []string {
	switch value := fields[key].(type) {
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
