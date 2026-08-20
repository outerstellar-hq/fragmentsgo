package search

import (
	"testing"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
)

type fakeRepo struct{ fragments []*fragmentsgo.Fragment }

func (f *fakeRepo) Load() error                                   { return nil }
func (f *fakeRepo) All() []*fragmentsgo.Fragment                  { return f.fragments }
func (f *fakeRepo) Everything() []*fragmentsgo.Fragment           { return f.fragments }
func (f *fakeRepo) BySlug(string) (*fragmentsgo.Fragment, error)  { return nil, fragmentsgo.ErrNotFound }
func (f *fakeRepo) ByURL(string) (*fragmentsgo.Fragment, error)   { return nil, fragmentsgo.ErrNotFound }
func (f *fakeRepo) UpdateStatus(string, fragmentsgo.Status) error { return nil }
func (f *fakeRepo) Schedule(string, time.Time) error              { return nil }
func (f *fakeRepo) Archive(string) error                          { return nil }

func TestSearchRanksTitleOverBody(t *testing.T) {
	fragments := []*fragmentsgo.Fragment{
		{Slug: "a", Title: "Go Concurrency", Preview: "about channels", BodyText: "nothing relevant"},
		{Slug: "b", Title: "Unrelated", Preview: "mentions go in passing", BodyText: "the word go appears here"},
	}
	engine := New(&fakeRepo{fragments: fragments})
	results := engine.Search(Options{Query: "go"})
	if len(results) < 2 {
		t.Fatalf("results = %v", results)
	}
	if results[0].Fragment.Slug != "a" {
		t.Fatalf("title match should rank first, got %s", results[0].Fragment.Slug)
	}
}

func TestSearchMultiTermPrefersAllTerms(t *testing.T) {
	fragments := []*fragmentsgo.Fragment{
		{Slug: "both", Title: "Go Testing Guide", BodyText: "testing go"},
		{Slug: "one", Title: "Go Guide", BodyText: "nothing else"},
	}
	engine := New(&fakeRepo{fragments: fragments})
	results := engine.Search(Options{Query: "go testing"})
	if len(results) != 2 || results[0].Fragment.Slug != "both" {
		t.Fatalf("ranking = %v", results)
	}
}

func TestFuzzyMatchesTypos(t *testing.T) {
	fragments := []*fragmentsgo.Fragment{
		{Slug: "a", Title: "Concurrency Patterns", BodyText: ""},
	}
	engine := New(&fakeRepo{fragments: fragments})
	if exact := engine.Search(Options{Query: "concurrency"}); len(exact) != 1 {
		t.Fatalf("exact failed: %v", exact)
	}
	if fuzzy := engine.Search(Options{Query: "concurency", Fuzzy: true}); len(fuzzy) != 1 {
		t.Fatalf("fuzzy failed: %v", fuzzy)
	}
	if strict := engine.Search(Options{Query: "concurency"}); len(strict) != 0 {
		t.Fatalf("without fuzzy should not match: %v", strict)
	}
}

func TestAutocompleteAndTagSearch(t *testing.T) {
	fragments := []*fragmentsgo.Fragment{
		{Slug: "a", Title: "Goroutines", Tags: []string{"concurrency"}},
		{Slug: "b", Title: "Channels", Tags: []string{"concurrency"}},
	}
	engine := New(&fakeRepo{fragments: fragments})
	if matches := engine.Autocomplete("gor", 5); len(matches) != 1 || matches[0] != "Goroutines" {
		t.Fatalf("autocomplete = %v", matches)
	}
	if byTag := engine.SearchByTag("concurrency"); len(byTag) != 2 {
		t.Fatalf("by tag = %v", byTag)
	}
}

func TestPhraseRequiresContiguousMatch(t *testing.T) {
	fragments := []*fragmentsgo.Fragment{
		{Slug: "phrase", Title: "Go Concurrency", BodyText: "go concurrency together"},
		{Slug: "split", Title: "Go and Concurrency", BodyText: "go elsewhere concurrency"},
	}
	engine := New(&fakeRepo{fragments: fragments})
	phrase := engine.Search(Options{Query: "go concurrency", Phrase: true})
	if len(phrase) != 1 || phrase[0].Fragment.Slug != "phrase" {
		t.Fatalf("phrase results = %v", phrase)
	}
}
