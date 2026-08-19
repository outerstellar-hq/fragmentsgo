// Package blog implements the blog engine over a dated fragment
// repository: paginated overviews, tag listings, archives, and
// previous/next navigation.
package blog

import (
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
)

// Engine serves blog listings over a repository of dated fragments.
type Engine struct {
	repository fragmentsgo.Repository
	pageSize   int
}

// New creates a blog engine. pageSize defaults to 10.
func New(repository fragmentsgo.Repository, pageSize int) *Engine {
	if pageSize < 1 {
		pageSize = 10
	}
	return &Engine{repository: repository, pageSize: pageSize}
}

// Post is one blog post with its neighbors computed for detail pages.
type Post struct {
	Fragment *fragmentsgo.Fragment
	Previous *fragmentsgo.Fragment
	Next     *fragmentsgo.Fragment
}

// OverviewPage is a paginated slice of the post listing.
type OverviewPage struct {
	Posts      []*fragmentsgo.Fragment
	Page       int
	TotalPages int
	TotalItems int
}

// Post returns a post with its previous (newer) and next (older) neighbors.
func (e *Engine) Post(slug string) (*Post, error) {
	fragment, err := e.repository.BySlug(slug)
	if err != nil {
		return nil, err
	}
	post := &Post{Fragment: fragment}
	posts := e.All()
	for index, candidate := range posts {
		if candidate == fragment {
			if index > 0 {
				post.Previous = posts[index-1]
			}
			if index+1 < len(posts) {
				post.Next = posts[index+1]
			}
			break
		}
	}
	return post, nil
}

// Overview returns the requested 1-based page of all posts.
func (e *Engine) Overview(page int) OverviewPage {
	return e.page(e.All(), page)
}

// All returns every post newest-first, enforcing ordering regardless of
// how the repository sorts.
func (e *Engine) All() []*fragmentsgo.Fragment {
	posts := e.repository.All()
	sorted := make([]*fragmentsgo.Fragment, len(posts))
	copy(sorted, posts)
	fragmentsgo.SortDated(sorted)
	return sorted
}

// Tag returns a page of posts carrying the tag.
func (e *Engine) Tag(tag string, page int) OverviewPage {
	var filtered []*fragmentsgo.Fragment
	for _, fragment := range e.repository.All() {
		if fragment.HasTag(tag) {
			filtered = append(filtered, fragment)
		}
	}
	return e.page(filtered, page)
}

// Tags returns the distinct tags across all posts, alphabetically.
func (e *Engine) Tags() []string {
	seen := map[string]bool{}
	var tags []string
	for _, fragment := range e.repository.All() {
		for _, tag := range fragment.Tags {
			key := lower(tag)
			if seen[key] {
				continue
			}
			seen[key] = true
			tags = append(tags, tag)
		}
	}
	sortStrings(tags)
	return tags
}

// ArchiveYear groups one year of posts.
type ArchiveYear struct {
	Year   int
	Posts  []*fragmentsgo.Fragment
	Months []ArchiveMonth
}

// ArchiveMonth groups one month of posts within a year.
type ArchiveMonth struct {
	Month time.Month
	Posts []*fragmentsgo.Fragment
}

// Years returns every year that has posts, newest first.
func (e *Engine) Years() []ArchiveYear {
	byYear := map[int][]*fragmentsgo.Fragment{}
	for _, fragment := range e.repository.All() {
		if fragment.Date.IsZero() {
			continue
		}
		byYear[fragment.Date.Year()] = append(byYear[fragment.Date.Year()], fragment)
	}
	var years []int
	for year := range byYear {
		years = append(years, year)
	}
	sortIntsDesc(years)
	var result []ArchiveYear
	for _, year := range years {
		result = append(result, ArchiveYear{Year: year, Posts: byYear[year], Months: monthsOf(byYear[year])})
	}
	return result
}

// Archive returns the posts of one year, optionally one month (1–12, 0 for
// the whole year).
func (e *Engine) Archive(year, month int) []*fragmentsgo.Fragment {
	var posts []*fragmentsgo.Fragment
	for _, fragment := range e.repository.All() {
		if fragment.Date.IsZero() || fragment.Date.Year() != year {
			continue
		}
		if month != 0 && int(fragment.Date.Month()) != month {
			continue
		}
		posts = append(posts, fragment)
	}
	return posts
}

func (e *Engine) page(posts []*fragmentsgo.Fragment, page int) OverviewPage {
	totalPages := (len(posts) + e.pageSize - 1) / e.pageSize
	if totalPages < 1 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	start := (page - 1) * e.pageSize
	if start >= len(posts) {
		return OverviewPage{Page: page, TotalPages: totalPages, TotalItems: len(posts)}
	}
	end := start + e.pageSize
	if end > len(posts) {
		end = len(posts)
	}
	return OverviewPage{
		Posts:      posts[start:end],
		Page:       page,
		TotalPages: totalPages,
		TotalItems: len(posts),
	}
}

func monthsOf(posts []*fragmentsgo.Fragment) []ArchiveMonth {
	byMonth := map[time.Month][]*fragmentsgo.Fragment{}
	var order []time.Month
	for _, fragment := range posts {
		month := fragment.Date.Month()
		if _, ok := byMonth[month]; !ok {
			order = append(order, month)
		}
		byMonth[month] = append(byMonth[month], fragment)
	}
	for i := 1; i < len(order); i++ {
		for j := i; j > 0 && order[j] > order[j-1]; j-- {
			order[j], order[j-1] = order[j-1], order[j]
		}
	}
	var months []ArchiveMonth
	for _, month := range order {
		months = append(months, ArchiveMonth{Month: month, Posts: byMonth[month]})
	}
	return months
}

func lower(value string) string {
	out := []rune(value)
	for i, r := range out {
		if r >= 'A' && r <= 'Z' {
			out[i] = r + 32
		}
	}
	return string(out)
}

func sortStrings(values []string) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] < values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}

func sortIntsDesc(values []int) {
	for i := 1; i < len(values); i++ {
		for j := i; j > 0 && values[j] > values[j-1]; j-- {
			values[j], values[j-1] = values[j-1], values[j]
		}
	}
}
