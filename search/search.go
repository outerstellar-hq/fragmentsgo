// Package search provides an in-memory scored search engine over fragment
// repositories — the pragmatic Go stand-in for fragments4k's Lucene engine.
// Terms score title ×5, tags ×3, preview ×2, body ×1; documents matching
// every term rank first, then by score, then by date.
package search

import (
	"sort"
	"strings"
	"sync"

	"github.com/outerstellar-hq/fragmentsgo"
)

// Options control one search call.
type Options struct {
	// Query is the free-text input (tokenized on non-alphanumerics).
	Query string
	// Phrase requires the full query as a contiguous substring.
	Phrase bool
	// Fuzzy additionally matches terms within edit distance 1.
	Fuzzy bool
	// MaxResults caps the returned list; 0 means unlimited.
	MaxResults int
}

// Result is one scored hit.
type Result struct {
	Fragment *fragmentsgo.Fragment
	Score    int
}

// Engine indexes fragments from one or more repositories in memory.
type Engine struct {
	mu      sync.RWMutex
	sources []fragmentsgo.Repository
}

// New creates an engine over the given repositories. Call Index after the
// repositories are loaded (and again after content changes).
func New(sources ...fragmentsgo.Repository) *Engine {
	return &Engine{sources: sources}
}

// Index refreshes the engine's view of its repositories. The engine reads
// through to the repositories, so this simply guarantees visibility of the
// latest Load; it returns the number of indexed fragments.
func (e *Engine) Index() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	count := 0
	for _, source := range e.sources {
		count += len(source.All())
	}
	return count
}

// Search runs the query and returns ranked results.
func (e *Engine) Search(options Options) []Result {
	terms := tokenize(options.Query)
	if len(terms) == 0 {
		return nil
	}
	type ranked struct {
		result  Result
		matched int
	}
	var rankedResults []ranked
	for _, source := range e.sources {
		for _, fragment := range source.All() {
			title := strings.ToLower(fragment.Title)
			tags := strings.ToLower(strings.Join(fragment.Tags, " "))
			preview := strings.ToLower(fragment.Preview)
			body := strings.ToLower(fragment.BodyText)
			if options.Phrase && !strings.Contains(strings.ToLower(queryHaystack(title, tags, preview, body)), strings.ToLower(strings.Join(terms, " "))) &&
				!strings.Contains(title+preview+body, strings.ToLower(options.Query)) {
				continue
			}
			score := 0
			matched := 0
			for _, term := range terms {
				termScore := 0
				if strings.Contains(title, term) {
					termScore += 5
				}
				if strings.Contains(tags, term) {
					termScore += 3
				}
				if strings.Contains(preview, term) {
					termScore += 2
				}
				termScore += countOccurrences(body, term)
				if options.Fuzzy {
					termScore += fuzzyHits(title, term)*5 + fuzzyHits(tags, term)*3
				}
				if termScore > 0 {
					matched++
				}
				score += termScore
			}
			if score == 0 {
				continue
			}
			rankedResults = append(rankedResults, ranked{
				result:  Result{Fragment: fragment, Score: score},
				matched: matched,
			})
		}
	}
	sort.SliceStable(rankedResults, func(i, j int) bool {
		if rankedResults[i].matched != rankedResults[j].matched {
			return rankedResults[i].matched > rankedResults[j].matched
		}
		if rankedResults[i].result.Score != rankedResults[j].result.Score {
			return rankedResults[i].result.Score > rankedResults[j].result.Score
		}
		a, b := rankedResults[i].result.Fragment.Date, rankedResults[j].result.Fragment.Date
		if a.Equal(b) {
			return rankedResults[i].result.Fragment.Title < rankedResults[j].result.Fragment.Title
		}
		return a.After(b)
	})
	var results []Result
	for _, entry := range rankedResults {
		if options.MaxResults > 0 && len(results) >= options.MaxResults {
			break
		}
		results = append(results, entry.result)
	}
	return results
}

// Autocomplete returns title/tag terms matching the prefix, up to limit.
func (e *Engine) Autocomplete(prefix string, limit int) []string {
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	if prefix == "" || limit <= 0 {
		return nil
	}
	seen := map[string]bool{}
	var matches []string
	for _, source := range e.sources {
		for _, fragment := range source.All() {
			candidates := append([]string{fragment.Title}, fragment.Tags...)
			for _, candidate := range candidates {
				key := strings.ToLower(candidate)
				if seen[key] || !strings.HasPrefix(key, prefix) {
					continue
				}
				seen[key] = true
				matches = append(matches, candidate)
				if len(matches) >= limit {
					return matches
				}
			}
		}
	}
	return matches
}

// SearchByTag returns every fragment carrying the tag.
func (e *Engine) SearchByTag(tag string) []Result {
	var results []Result
	for _, source := range e.sources {
		for _, fragment := range source.All() {
			if fragment.HasTag(tag) {
				results = append(results, Result{Fragment: fragment, Score: 3})
			}
		}
	}
	return results
}

func queryHaystack(title, tags, preview, body string) string {
	return title + " " + tags + " " + preview + " " + body
}

func tokenize(query string) []string {
	var terms []string
	seen := map[string]bool{}
	for _, term := range strings.Fields(strings.ToLower(strings.TrimSpace(query))) {
		term = strings.Trim(term, ".,!?;:\"'()[]")
		if len(term) >= 2 && !seen[term] {
			seen[term] = true
			terms = append(terms, term)
		}
	}
	return terms
}

func countOccurrences(haystack, needle string) int {
	if needle == "" {
		return 0
	}
	count := 0
	for index := 0; ; {
		index = strings.Index(haystack[index:], needle)
		if index < 0 {
			break
		}
		count++
		if count >= 10 {
			break
		}
		index++
	}
	return count
}

// fuzzyHits counts words in haystack within edit distance 1 of the term.
func fuzzyHits(haystack, term string) int {
	words := strings.Fields(haystack)
	if len(words) > 200 {
		words = words[:200]
	}
	hits := 0
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'()[]")
		if editDistanceAtMost1(word, term) {
			hits++
		}
	}
	return hits
}

func editDistanceAtMost1(a, b string) bool {
	if a == b {
		return true
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			continue
		}
		if len(a) == len(b) {
			return a[i+1:] == b[i+1:]
		}
		return a[i:] == b[i+1:]
	}
	return true
}
