package fragmentsgo_test

import (
	"strings"
	"testing"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
	fragmentsrss "github.com/outerstellar-hq/fragmentsgo/rss"
	"github.com/outerstellar-hq/fragmentsgo/sitemap"
)

func TestRSSFeed(t *testing.T) {
	fragments := []*fragmentsgo.Fragment{
		{Title: "First", Slug: "first", URL: "/blog/2026/01/first", Preview: "One", Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{Title: "Second", Slug: "second", URL: "/blog/2026/02/second", Preview: "Two"},
	}
	feed := fragmentsrss.Build(fragmentsrss.Channel{
		Title: "Demo", Link: "https://demo.test/blog", Description: "Feed",
	}, fragments)
	data, err := feed.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"<rss version=\"2.0\">",
		"<title>Demo</title>",
		"<link>https://demo.test/blog/2026/01/first</link>",
		"<pubDate>Thu, 01 Jan 2026 00:00:00 +0000</pubDate>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("feed missing %q:\n%s", want, text)
		}
	}
}

func TestSitemapBuilder(t *testing.T) {
	builder := sitemap.New("https://demo.test/").Add("/").AddAll([]string{"/blog", "projects/x"}).Add("https://other.example/y")
	data, err := builder.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"<loc>https://demo.test/</loc>",
		"<loc>https://demo.test/blog</loc>",
		"<loc>https://demo.test/projects/x</loc>",
		"<loc>https://other.example/y</loc>",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("sitemap missing %q:\n%s", want, text)
		}
	}
}
