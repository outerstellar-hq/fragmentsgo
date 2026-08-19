// Command example is a minimal fragmentsgo site: a blog, an about page,
// search, RSS, and a sitemap rendered through html/template. Run it from
// the repository root:
//
//	go run ./cmd/example
//
// then open http://localhost:8080.
package main

import (
	"embed"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
	"github.com/outerstellar-hq/fragmentsgo/blog"
	"github.com/outerstellar-hq/fragmentsgo/httpadapter"
	"github.com/outerstellar-hq/fragmentsgo/search"
	"github.com/outerstellar-hq/fragmentsgo/static"
)

//go:embed templates/*.html
var files embed.FS

func main() {
	blogRepo := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
		Path: "cmd/example/content/blog",
		URLBuilder: func(f *fragmentsgo.Fragment) string {
			return "/blog/" + f.Date.Format("2006/01") + "/" + f.Slug
		},
	})
	pageRepo := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
		Path:    "cmd/example/content/pages",
		Ordered: true,
	})
	if err := blogRepo.Load(); err != nil {
		log.Fatal(err)
	}
	if err := pageRepo.Load(); err != nil {
		log.Fatal(err)
	}

	templates := template.Must(template.New("").Funcs(template.FuncMap{
		"inc": func(v int) int { return v + 1 },
		"dec": func(v int) int { return v - 1 },
	}).ParseFS(files, "templates/*.html"))
	renderer := httpadapter.RendererFunc(func(w http.ResponseWriter, r *http.Request, name string, data any) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := templates.ExecuteTemplate(w, name+".html", data); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	})

	adapter := httpadapter.New(httpadapter.Config{
		Blog:      blog.New(blogRepo, 10),
		Static:    static.New(pageRepo),
		Search:    search.New(blogRepo, pageRepo),
		Renderer:  renderer,
		SiteTitle: "fragmentsgo example",
		SiteURL:   "http://localhost:8080",
	})

	mux := http.NewServeMux()
	adapter.Mount(mux)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/blog", http.StatusFound)
	})
	mux.HandleFunc("GET /about", adapter.StaticPageHandler("about"))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("fragmentsgo example listening on :%s", port)
	log.Fatal(server.ListenAndServe())
}
