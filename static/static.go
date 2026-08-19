// Package static implements the engine for undated, ordered content:
// standalone pages and custom sections like projects or articles.
package static

import (
	"strings"

	"github.com/outerstellar-hq/fragmentsgo"
)

// Engine serves ordered fragments (pages, projects, article sections).
type Engine struct {
	repository fragmentsgo.Repository
}

// New creates a static-content engine. The repository should use Ordered
// listing (Order then Title).
func New(repository fragmentsgo.Repository) *Engine {
	return &Engine{repository: repository}
}

// Page returns a single fragment by slug.
func (e *Engine) Page(slug string) (*fragmentsgo.Fragment, error) {
	return e.repository.BySlug(slug)
}

// PageByURL returns a single fragment by its public URL.
func (e *Engine) PageByURL(url string) (*fragmentsgo.Fragment, error) {
	return e.repository.ByURL(url)
}

// All returns every fragment in listing order.
func (e *Engine) All() []*fragmentsgo.Fragment {
	return e.repository.All()
}

// Tagged returns the fragments carrying a tag, preserving listing order.
func (e *Engine) Tagged(tag string) []*fragmentsgo.Fragment {
	var filtered []*fragmentsgo.Fragment
	for _, fragment := range e.repository.All() {
		if fragment.HasTag(tag) {
			filtered = append(filtered, fragment)
		}
	}
	return filtered
}

// IsIndex reports whether the fragment acts as the section's index page
// (slug "index" or "home"), rendered at the base URL.
func IsIndex(fragment *fragmentsgo.Fragment) bool {
	slug := strings.ToLower(fragment.Slug)
	return slug == "index" || slug == "home"
}
