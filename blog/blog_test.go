package blog

import (
	"testing"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
)

type fakeRepo struct{ fragments []*fragmentsgo.Fragment }

func (f *fakeRepo) Load() error                  { return nil }
func (f *fakeRepo) All() []*fragmentsgo.Fragment { return f.fragments }
func (f *fakeRepo) Everything() []*fragmentsgo.Fragment { return f.fragments }
func (f *fakeRepo) BySlug(slug string) (*fragmentsgo.Fragment, error) {
	for _, fragment := range f.fragments {
		if fragment.Slug == slug {
			return fragment, nil
		}
	}
	return nil, fragmentsgo.ErrNotFound
}
func (f *fakeRepo) ByURL(url string) (*fragmentsgo.Fragment, error) {
	return nil, fragmentsgo.ErrNotFound
}
func (f *fakeRepo) UpdateStatus(string, fragmentsgo.Status) error { return nil }
func (f *fakeRepo) Schedule(string, time.Time) error              { return nil }
func (f *fakeRepo) Archive(string) error                          { return nil }

func dated(slug string, date time.Time, tags ...string) *fragmentsgo.Fragment {
	return &fragmentsgo.Fragment{Slug: slug, Title: slug, Date: date, Tags: tags}
}

func TestOverviewPaginates(t *testing.T) {
	var posts []*fragmentsgo.Fragment
	for i := 1; i <= 12; i++ {
		posts = append(posts, dated(string(rune('a'+i)), time.Date(2026, 1, i, 0, 0, 0, 0, time.UTC)))
	}
	repo := &fakeRepo{fragments: posts}
	engine := New(repo, 5)
	page1 := engine.Overview(1)
	if len(page1.Posts) != 5 || page1.TotalPages != 3 || page1.TotalItems != 12 {
		t.Fatalf("page1 = %+v", page1)
	}
	if page1.Posts[0].Slug != "m" {
		t.Fatalf("newest first broken: %s", page1.Posts[0].Slug)
	}
	page3 := engine.Overview(3)
	if len(page3.Posts) != 2 {
		t.Fatalf("page3 posts = %d", len(page3.Posts))
	}
	if oversized := engine.Overview(99); oversized.Page != 3 {
		t.Fatalf("page clamping failed: %d", oversized.Page)
	}
}

func TestPostAdjacent(t *testing.T) {
	posts := []*fragmentsgo.Fragment{
		dated("old", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)),
		dated("mid", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)),
		dated("new", time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)),
	}
	engine := New(&fakeRepo{fragments: posts}, 10)
	post, err := engine.Post("mid")
	if err != nil {
		t.Fatal(err)
	}
	if post.Previous == nil || post.Previous.Slug != "new" || post.Next == nil || post.Next.Slug != "old" {
		t.Fatalf("adjacent wrong: prev=%v next=%v", post.Previous, post.Next)
	}
}

func TestTagAndArchive(t *testing.T) {
	posts := []*fragmentsgo.Fragment{
		dated("jan-go", time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC), "go"),
		dated("feb-rust", time.Date(2026, 2, 10, 0, 0, 0, 0, time.UTC), "rust"),
		dated("mar-go", time.Date(2026, 3, 10, 0, 0, 0, 0, time.UTC), "go"),
	}
	engine := New(&fakeRepo{fragments: posts}, 10)
	if tagged := engine.Tag("GO", 1); tagged.TotalItems != 2 {
		t.Fatalf("tag count = %d", tagged.TotalItems)
	}
	years := engine.Years()
	if len(years) != 1 || len(years[0].Months) != 3 {
		t.Fatalf("years = %+v", years)
	}
	if posts := engine.Archive(2026, 2); len(posts) != 1 || posts[0].Slug != "feb-rust" {
		t.Fatalf("archive month = %v", posts)
	}
	if posts := engine.Archive(2026, 0); len(posts) != 3 {
		t.Fatalf("archive year = %d posts", len(posts))
	}
}
