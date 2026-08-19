package httpadapter

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
	"github.com/outerstellar-hq/fragmentsgo/blog"
	"github.com/outerstellar-hq/fragmentsgo/search"
	"github.com/outerstellar-hq/fragmentsgo/static"
)

func writeContent(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newTestAdapter(t *testing.T) (*Adapter, *strings.Builder) {
	t.Helper()
	root := t.TempDir()
	writeContent(t, filepath.Join(root, "blog"), "2026-01-01-first.md",
		"---\ntitle: First\ndate: 2026-01-01\n---\nFirst body")
	writeContent(t, filepath.Join(root, "blog"), "2026-02-01-second.md",
		"---\ntitle: Second\ndate: 2026-02-01\ntags: [go]\n---\nSecond body")
	writeContent(t, filepath.Join(root, "pages"), "about.md",
		"---\ntitle: About\norder: 1\n---\nAbout body")

	blogRepo := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
		Path: filepath.Join(root, "blog"),
		URLBuilder: func(f *fragmentsgo.Fragment) string {
			return "/blog/" + f.Date.Format("2006/01") + "/" + f.Slug
		},
		Now: func() time.Time { return time.Date(2026, 8, 19, 0, 0, 0, 0, time.UTC) },
	})
	pageRepo := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
		Path:    filepath.Join(root, "pages"),
		Ordered: true,
	})
	if err := blogRepo.Load(); err != nil {
		t.Fatal(err)
	}
	if err := pageRepo.Load(); err != nil {
		t.Fatal(err)
	}
	rendered := &strings.Builder{}
	renderer := RendererFunc(func(w http.ResponseWriter, r *http.Request, name string, data any) {
		rendered.Reset()
		fmt.Fprintf(rendered, "template=%s", name)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(rendered.String()))
	})
	adapter := New(Config{
		Blog:      blog.New(blogRepo, 10),
		Static:    static.New(pageRepo),
		Search:    search.New(blogRepo, pageRepo),
		Renderer:  renderer,
		SiteTitle: "Demo",
		SiteURL:   "https://demo.test",
	})
	return adapter, rendered
}

func TestAdapterRoutes(t *testing.T) {
	adapter, rendered := newTestAdapter(t)
	mux := http.NewServeMux()
	adapter.Mount(mux)
	mux.HandleFunc("GET /about", adapter.StaticPageHandler("about"))

	cases := []struct {
		path     string
		status   int
		template string
	}{
		{"/blog", 200, "blog"},
		{"/blog/tag/go", 200, "blog"},
		{"/blog/archive", 200, "archive"},
		{"/blog/archive/2026", 200, "archive"},
		{"/blog/archive/2026/1", 200, "archive"},
		{"/blog/2026/01/first", 200, "post"},
		{"/blog/2026/01/missing", 404, ""},
		{"/search?q=second", 200, "search"},
		{"/about", 200, "static"},
	}
	for _, testCase := range cases {
		recorder := httptest.NewRecorder()
		mux.ServeHTTP(recorder, httptest.NewRequest("GET", testCase.path, nil))
		if recorder.Code != testCase.status {
			t.Fatalf("%s status = %d, want %d", testCase.path, recorder.Code, testCase.status)
		}
		if testCase.template != "" && !strings.Contains(rendered.String(), "template="+testCase.template) {
			t.Fatalf("%s rendered %q, want template %s", testCase.path, rendered.String(), testCase.template)
		}
	}
}

func TestAdapterFeeds(t *testing.T) {
	adapter, _ := newTestAdapter(t)
	mux := http.NewServeMux()
	adapter.Mount(mux)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/rss.xml", nil))
	if recorder.Code != 200 || !strings.Contains(recorder.Body.String(), "<item>") {
		t.Fatalf("rss = %d %s", recorder.Code, recorder.Body.String())
	}
	if contentType := recorder.Header().Get("Content-Type"); !strings.Contains(contentType, "rss+xml") {
		t.Fatalf("rss content type = %s", contentType)
	}

	recorder = httptest.NewRecorder()
	mux.ServeHTTP(recorder, httptest.NewRequest("GET", "/sitemap.xml", nil))
	body := recorder.Body.String()
	if recorder.Code != 200 || !strings.Contains(body, "/blog/2026/01/first") || !strings.Contains(body, "/about") {
		t.Fatalf("sitemap = %d %s", recorder.Code, body)
	}
}
