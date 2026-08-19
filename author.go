package fragmentsgo

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Author is a content author profile loaded from a .author.yml file.
type Author struct {
	ID          string            `yaml:"id"`
	Name        string            `yaml:"name"`
	Slug        string            `yaml:"slug"`
	Bio         string            `yaml:"bio"`
	GitHub      string            `yaml:"github"`
	Twitter     string            `yaml:"twitter"`
	SocialLinks map[string]string `yaml:"socialLinks"`
}

// SocialEntries returns the author's social links with GitHub/Twitter
// expanded to full URLs, in a stable order.
func (a *Author) SocialEntries() []AuthorLink {
	urls := map[string]string{}
	if a.GitHub != "" {
		urls["github"] = "https://github.com/" + a.GitHub
	}
	if a.Twitter != "" {
		urls["twitter"] = "https://x.com/" + a.Twitter
	}
	for name, url := range a.SocialLinks {
		urls[strings.ToLower(name)] = url
	}
	order := make([]string, 0, len(urls))
	for name := range urls {
		order = append(order, name)
	}
	sortStrings(order)
	var entries []AuthorLink
	for _, name := range order {
		entries = append(entries, AuthorLink{Name: name, URL: urls[name]})
	}
	return entries
}

// AuthorLink is one outbound social profile link.
type AuthorLink struct {
	Name string
	URL  string
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

// AuthorRepository loads author profiles from a directory of .author.yml
// files and resolves them by id, slug, or display name.
type AuthorRepository struct {
	authors map[string]*Author
}

// NewAuthorRepository loads every *.author.yml file under root. A missing
// directory yields an empty repository.
func NewAuthorRepository(root string) (*AuthorRepository, error) {
	repository := &AuthorRepository{authors: map[string]*Author{}}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return repository, nil
	}
	if err != nil {
		return nil, fmt.Errorf("fragmentsgo: read authors: %w", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".author.yml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			return nil, fmt.Errorf("fragmentsgo: read author %s: %w", entry.Name(), err)
		}
		var author Author
		if err := yaml.Unmarshal(data, &author); err != nil {
			return nil, fmt.Errorf("fragmentsgo: parse author %s: %w", entry.Name(), err)
		}
		if author.Name == "" {
			author.Name = strings.TrimSuffix(entry.Name(), ".author.yml")
		}
		if author.ID == "" {
			author.ID = author.Name
		}
		if author.Slug == "" {
			author.Slug = Slugify(author.Name)
		}
		repository.authors[author.ID] = &author
		repository.authors[author.Slug] = &author
		repository.authors[strings.ToLower(author.Name)] = &author
	}
	return repository, nil
}

// Resolve finds an author by id, slug, or display name; nil when the name
// is empty or unknown.
func (r *AuthorRepository) Resolve(name string) *Author {
	if name == "" {
		return nil
	}
	return r.authors[strings.ToLower(strings.TrimSpace(name))]
}

// All returns every author profile ordered by name.
func (r *AuthorRepository) All() []*Author {
	var authors []*Author
	seen := map[*Author]bool{}
	for _, author := range r.authors {
		if seen[author] {
			continue
		}
		seen[author] = true
		authors = append(authors, author)
	}
	names := make([]string, len(authors))
	byName := map[string]*Author{}
	for i, author := range authors {
		names[i] = author.Name
		byName[author.Name] = author
	}
	sortStrings(names)
	ordered := make([]*Author, 0, len(authors))
	for _, name := range names {
		ordered = append(ordered, byName[name])
	}
	return ordered
}
