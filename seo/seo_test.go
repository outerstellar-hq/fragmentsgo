package seo

import (
	"strings"
	"testing"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
)

func TestForFragmentHeadTags(t *testing.T) {
	site := Site{Name: "Demo", URL: "https://demo.test", Description: "A demo"}
	fragment := &fragmentsgo.Fragment{
		Title:   "Hello Post",
		Slug:    "hello",
		URL:     "/blog/2026/03/hello",
		Preview: "A post about hello.",
		Date:    time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
		Author:  "jane",
	}
	meta := ForFragment(site, fragment, "article").
		WithJSONLD(BlogPostingJSONLD(site, fragment), BreadcrumbJSONLD(site, [][2]string{{"Home", "/"}, {"Blog", "/blog"}}))
	tags := meta.HeadTags("google-code", "bing-code")
	for _, want := range []string{
		`<meta property="og:title" content="Hello Post">`,
		`<meta property="og:type" content="article">`,
		`<link rel="canonical" href="https://demo.test/blog/2026/03/hello">`,
		`<meta property="article:published_time" content="2026-03-01T00:00:00Z">`,
		`<meta name="twitter:card" content="summary">`,
		`<meta name="google-site-verification" content="google-code">`,
		`<meta name="msvalidate.01" content="bing-code">`,
		`"@type":"BlogPosting"`,
		`"@type":"BreadcrumbList"`,
	} {
		if !strings.Contains(tags, want) {
			t.Fatalf("head tags missing %q in:\n%s", want, tags)
		}
	}
}

func TestForPageRobots(t *testing.T) {
	site := Site{Name: "Demo", URL: "https://demo.test"}
	noindex := ForPage(site, "Search", "search", "/search", RobotsNoIndex)
	if tags := noindex.HeadTags("", ""); !strings.Contains(tags, `<meta name="robots" content="noindex">`) {
		t.Fatalf("noindex missing: %s", tags)
	}
	indexed := ForPage(site, "Home", "home", "/", RobotsIndex)
	if tags := indexed.HeadTags("", ""); strings.Contains(tags, "robots") {
		t.Fatalf("indexed page should have no robots tag: %s", tags)
	}
}

func TestWebSiteJSONLDSearchAction(t *testing.T) {
	site := Site{Name: "Demo", URL: "https://demo.test"}
	payload := WebSiteJSONLD(site, "/search?")
	if !strings.Contains(payload, `"target":"https://demo.test/search?q={search_term_string}"`) {
		t.Fatalf("payload = %s", payload)
	}
}
