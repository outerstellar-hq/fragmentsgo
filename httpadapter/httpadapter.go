// Package httpadapter mounts a fragmentsgo content site on any net/http
// mux: blog routes (paged listing, tags, dated posts, archive), search,
// static pages, and the rss.xml and sitemap.xml endpoints. Rendering stays
// consumer-owned — provide a Renderer that writes your templates.
package httpadapter

import (
	"net/http"
	"strconv"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
	"github.com/outerstellar-hq/fragmentsgo/blog"
	"github.com/outerstellar-hq/fragmentsgo/rss"
	"github.com/outerstellar-hq/fragmentsgo/search"
	"github.com/outerstellar-hq/fragmentsgo/sitemap"
	"github.com/outerstellar-hq/fragmentsgo/static"
)

// Renderer draws a named template with page data. Template names follow the
// fragments4k conventions: "blog", "archive", "search", "static".
type Renderer interface {
	Render(w http.ResponseWriter, r *http.Request, name string, data any)
}

// RendererFunc adapts a function to Renderer.
type RendererFunc func(w http.ResponseWriter, r *http.Request, name string, data any)

// Render implements Renderer.
func (f RendererFunc) Render(w http.ResponseWriter, r *http.Request, name string, data any) {
	f(w, r, name, data)
}

// Config wires the adapter.
type Config struct {
	Blog      *blog.Engine
	Static    *static.Engine
	Search    *search.Engine
	Renderer  Renderer
	SiteTitle string
	SiteURL   string
	// SearchPath is the search page path ("/search").
	SearchPath string
	// Now overrides time (tests).
	Now func() time.Time
}

// Adapter mounts content routes on a mux.
type Adapter struct {
	config Config
}

// New creates the adapter.
func New(config Config) *Adapter {
	if config.SearchPath == "" {
		config.SearchPath = "/search"
	}
	if config.Now == nil {
		config.Now = time.Now
	}
	return &Adapter{config: config}
}

// OverviewVM is the blog listing view model.
type OverviewVM struct {
	Page  blog.OverviewPage
	Query string
}

// PostVM is the blog post detail view model.
type PostVM struct {
	Post *blog.Post
}

// ArchiveVM is the archive view model (Year/Month zero for the index).
type ArchiveVM struct {
	Years []blog.ArchiveYear
	Year  int
	Month int
	Posts []*fragmentsgo.Fragment
}

// SearchVM is the search page view model.
type SearchVM struct {
	Query   string
	Results []search.Result
}

// StaticVM is the static page view model.
type StaticVM struct {
	Fragment *fragmentsgo.Fragment
}

// Mount registers all routes on the mux.
func (a *Adapter) Mount(mux *http.ServeMux) {
	if a.config.Blog != nil {
		mux.HandleFunc("GET /blog", a.blogOverview)
		mux.HandleFunc("GET /blog/tag/{tag}", a.blogTag)
		mux.HandleFunc("GET /blog/archive", a.archiveIndex)
		mux.HandleFunc("GET /blog/archive/{year}", a.archiveYear)
		mux.HandleFunc("GET /blog/archive/{year}/{month}", a.archiveMonth)
		mux.HandleFunc("GET /blog/{year}/{month}/{slug}", a.blogPost)
		mux.HandleFunc("GET /rss.xml", a.rssFeed)
	}
	if a.config.Search != nil {
		mux.HandleFunc("GET "+a.config.SearchPath, a.searchPage)
	}
	if a.config.Static != nil {
		mux.HandleFunc("GET /sitemap.xml", a.sitemapFile)
	}
}

func (a *Adapter) blogOverview(w http.ResponseWriter, r *http.Request) {
	a.config.Renderer.Render(w, r, "blog", OverviewVM{Page: a.config.Blog.Overview(pageParam(r))})
}

func (a *Adapter) blogTag(w http.ResponseWriter, r *http.Request) {
	tag := r.PathValue("tag")
	a.config.Renderer.Render(w, r, "blog", OverviewVM{
		Page:  a.config.Blog.Tag(tag, pageParam(r)),
		Query: tag,
	})
}

func (a *Adapter) blogPost(w http.ResponseWriter, r *http.Request) {
	post, err := a.config.Blog.Post(r.PathValue("slug"))
	if err == fragmentsgo.ErrNotFound {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		http.Error(w, "post unavailable", http.StatusInternalServerError)
		return
	}
	a.config.Renderer.Render(w, r, "post", PostVM{Post: post})
}

func (a *Adapter) archiveIndex(w http.ResponseWriter, r *http.Request) {
	a.config.Renderer.Render(w, r, "archive", ArchiveVM{Years: a.config.Blog.Years()})
}

func (a *Adapter) archiveYear(w http.ResponseWriter, r *http.Request) {
	year, err := strconv.Atoi(r.PathValue("year"))
	if err != nil {
		http.NotFound(w, r)
		return
	}
	a.config.Renderer.Render(w, r, "archive", ArchiveVM{Year: year, Posts: a.config.Blog.Archive(year, 0)})
}

func (a *Adapter) archiveMonth(w http.ResponseWriter, r *http.Request) {
	year, yearErr := strconv.Atoi(r.PathValue("year"))
	month, monthErr := strconv.Atoi(r.PathValue("month"))
	if yearErr != nil || monthErr != nil || month < 1 || month > 12 {
		http.NotFound(w, r)
		return
	}
	a.config.Renderer.Render(w, r, "archive", ArchiveVM{Year: year, Month: month, Posts: a.config.Blog.Archive(year, month)})
}

func (a *Adapter) searchPage(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	results := a.config.Search.Search(search.Options{Query: query})
	a.config.Renderer.Render(w, r, "search", SearchVM{Query: query, Results: results})
}

func (a *Adapter) rssFeed(w http.ResponseWriter, r *http.Request) {
	feed := rss.Build(rss.Channel{
		Title:       a.config.SiteTitle,
		Link:        a.config.SiteURL,
		Description: a.config.SiteTitle,
	}, a.config.Blog.All())
	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	if err := feed.Write(w); err != nil {
		http.Error(w, "feed unavailable", http.StatusInternalServerError)
	}
}

func (a *Adapter) sitemapFile(w http.ResponseWriter, r *http.Request) {
	builder := sitemap.New(a.config.SiteURL).Add("/")
	for _, fragment := range a.config.Blog.All() {
		builder.Add(fragment.URL)
	}
	for _, fragment := range a.config.Static.All() {
		builder.Add(fragment.URL)
	}
	data, err := builder.Bytes()
	if err != nil {
		http.Error(w, "sitemap unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write(data)
}

// StaticPageHandler returns a handler rendering one static page by slug —
// mount it per page or wire it behind a catch-all route.
func (a *Adapter) StaticPageHandler(slug string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		fragment, err := a.config.Static.Page(slug)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		a.config.Renderer.Render(w, r, "static", StaticVM{Fragment: fragment})
	}
}

func pageParam(r *http.Request) int {
	if parsed, err := strconv.Atoi(r.URL.Query().Get("page")); err == nil && parsed > 0 {
		return parsed
	}
	return 1
}
