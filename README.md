# fragmentsgo

A framework-agnostic Markdown content engine for Go — the counterpart of
[Kotlin's fragments4k](https://github.com/rygel/fragments4k). Point it at a
directory of Markdown files with YAML front matter and get a blog, static
pages, search, RSS, sitemaps, reading time, author profiles, and SEO
metadata. Rendering and routing stay yours; everything else is here.

## Quick start

```go
blogRepo := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
    Path: "./content/blog",
    URLBuilder: func(f *fragmentsgo.Fragment) string {
        return "/blog/" + f.Date.Format("2006/01") + "/" + f.Slug
    },
})
pageRepo := fragmentsgo.NewFileSystemRepository(fragmentsgo.RepositoryOptions{
    Path:    "./content/pages",
    Ordered: true,
})
_ = blogRepo.Load()
_ = pageRepo.Load()

adapter := httpadapter.New(httpadapter.Config{
    Blog:      blog.New(blogRepo, 10),
    Static:    static.New(pageRepo),
    Search:    search.New(blogRepo, pageRepo),
    Renderer:  renderer, // your templates: blog, post, archive, search, static
    SiteTitle: "My Site",
    SiteURL:   "https://example.com",
})
mux := http.NewServeMux()
adapter.Mount(mux) // /blog, /blog/tag/{tag}, /blog/archive, /rss.xml, /sitemap.xml, /search
```

See `cmd/example` for a complete runnable site.

## Packages

| Package | Purpose |
|---|---|
| `fragmentsgo` | `Fragment` model with typed front-matter accessors, lifecycle statuses, Markdown parser with sanitizer profiles, file-system repository (load/reload, status rewrites), author repository, reading time |
| `blog` | Paginated overviews, tag listings, year/month archives, previous/next |
| `static` | Ordered pages and custom sections (projects, articles) |
| `search` | Scored in-memory search (`Search`, `Autocomplete`, `SearchByTag`) with phrase and fuzzy options — the pragmatic stand-in for Lucene |
| `seo` | Open Graph/Twitter tags, canonical URLs, robots directives, JSON-LD (Organization, WebSite, BlogPosting, Breadcrumb) |
| `rss` | RSS 2.0 feed builder |
| `sitemap` | sitemap.xml builder |
| `httpadapter` | Mounts everything on any `*http.ServeMux` |
| `reload` | Dependency-free development watcher: polls directories for Markdown changes and fires a debounced callback |
| `cmd/fragmentsgo` | CLI: scaffold fragments (`new`) and validate directories (`validate`) |

## Development workflow

```bash
# Scaffold a draft (slug from the title; refuses to overwrite)
go run ./cmd/fragmentsgo new -dir ./content/blog "My Next Post"

# Validate front matter, slug collisions, and URL clashes
go run ./cmd/fragmentsgo validate ./content/blog ./content/articles

# Hot-reload content while developing (poll-based, stdlib only)
watcher := reload.Watch(ctx, []string{"content"}, reload.Options{Interval: 2 * time.Second}, func() {
    _ = store.Refresh() // or repo.Load()
})
```

## Content model

```yaml
---
title: Hello World
slug: hello            # optional, derived from title
date: 2026-03-01
tags: [go, testing]
author: jane           # matches an author profile
image: /images/hero.png
preview: Short summary # optional, first paragraph by default
template: wide         # optional alternate template
order: 3               # ordering for pages/sections
status: draft          # draft|review|approved|published|archived
publishAt: 2026-06-01T09:00:00Z  # future date → scheduled
expiresAt: 2027-01-01  # past date → expired
githubRepo: owner/repo # any custom field, read via GetString/GetInt/...
---
Body in **Markdown**.
```

Fragments are visible only when they resolve to `published` (explicitly, or
by default when no restrictive fields are set). The repository rewrites
front matter for lifecycle moves:

```go
_ = repo.UpdateStatus("hello", fragmentsgo.StatusPublished)
_ = repo.Schedule("hello", time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC))
_ = repo.Archive("hello")
```

## Sanitizer profiles

`SanitizerRelaxedTrustedAuthor` (default) keeps rich formatting while
stripping scripts and event handlers — for content you author.
`SanitizerStrict` reduces to basic inline formatting — for untrusted
contributions.

## Status of this port

Covers the fragments4k core: content repository, blog/static engines,
search, SEO, RSS, sitemap, authors, and an `net/http` adapter (Go needs
only one). Not ported: Lucene-backed index (replaced by the scored
in-memory engine), CLI scaffolding, live reload, chat/social modules,
image optimization, and per-framework adapters.

## License

MIT — see [LICENSE](LICENSE).
