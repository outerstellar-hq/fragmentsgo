package fragmentsgo

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func fixedNow() time.Time { return time.Date(2026, 8, 19, 12, 0, 0, 0, time.UTC) }

func TestRepositoryLoadsFrontMatterAndRenders(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "hello.md", `---
title: Hello World
slug: hello
date: 2026-03-01
tags: [go, testing]
author: alex
githubRepo: owner/repo
stars: 42
---
# Heading

First paragraph with [link](https://example.com).
`)
	repo := NewFileSystemRepository(RepositoryOptions{Path: dir, BaseURL: "/blog", Now: fixedNow})
	if err := repo.Load(); err != nil {
		t.Fatal(err)
	}
	fragment, err := repo.BySlug("hello")
	if err != nil {
		t.Fatal(err)
	}
	if fragment.Title != "Hello World" || fragment.URL != "/blog/hello" {
		t.Fatalf("unexpected fragment %+v", fragment)
	}
	if fragment.Status != StatusPublished || !fragment.Visible() {
		t.Fatalf("status = %s", fragment.Status)
	}
	if fragment.GetString("githubRepo") != "owner/repo" || fragment.GetInt("stars") != 42 {
		t.Fatalf("typed accessors failed: %v", fragment.Fields)
	}
	if len(fragment.Tags) != 2 || !fragment.HasTag("GO") {
		t.Fatalf("tags = %v", fragment.Tags)
	}
	if !strings.Contains(string(fragment.HTML), "<h1") {
		t.Fatalf("markdown not rendered: %s", fragment.HTML)
	}
	if !strings.Contains(fragment.Preview, "First paragraph") {
		t.Fatalf("preview = %q", fragment.Preview)
	}
	if fragment.ReadingTime < 1 {
		t.Fatalf("reading time = %d", fragment.ReadingTime)
	}
}

func TestStatusLifecycle(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "draft.md", "---\ntitle: Draft Post\nstatus: draft\n---\nBody")
	writeContent(t, dir, "future.md", "---\ntitle: Future Post\npublishAt: 2030-01-01\n---\nBody")
	writeContent(t, dir, "gone.md", "---\ntitle: Gone Post\nexpiresAt: 2020-01-01\n---\nBody")
	writeContent(t, dir, "archived.md", "---\ntitle: Archived Post\nstatus: archived\n---\nBody")
	writeContent(t, dir, "live.md", "---\ntitle: Live Post\n---\nBody")
	repo := NewFileSystemRepository(RepositoryOptions{Path: dir, Now: fixedNow})
	if err := repo.Load(); err != nil {
		t.Fatal(err)
	}
	includingInvisible := NewFileSystemRepository(RepositoryOptions{Path: dir, Now: fixedNow, IncludeInvisible: true})
	if err := includingInvisible.Load(); err != nil {
		t.Fatal(err)
	}
	expectations := map[string]Status{
		"draft-post":    StatusDraft,
		"future-post":   StatusScheduled,
		"gone-post":     StatusExpired,
		"archived-post": StatusArchived,
		"live-post":     StatusPublished,
	}
	for slug, want := range expectations {
		fragment, err := includingInvisible.BySlug(slug)
		if err != nil {
			t.Fatalf("%s: %v", slug, err)
		}
		if fragment.Status != want {
			t.Fatalf("%s status = %s, want %s", slug, fragment.Status, want)
		}
	}
	if got := len(repo.All()); got != 1 {
		t.Fatalf("visible fragments = %d, want 1 (only live)", got)
	}
}

func TestUpdateStatusAndScheduleRewriteFiles(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "post.md", "---\ntitle: My Post\n---\nBody")
	repo := NewFileSystemRepository(RepositoryOptions{Path: dir, Now: fixedNow})
	if err := repo.Load(); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpdateStatus("my-post", StatusDraft); err != nil {
		t.Fatal(err)
	}
	invisible := NewFileSystemRepository(RepositoryOptions{Path: dir, Now: fixedNow, IncludeInvisible: true})
	if err := invisible.Load(); err != nil {
		t.Fatal(err)
	}
	if fragment, err := invisible.BySlug("my-post"); err != nil || fragment.Status != StatusDraft {
		t.Fatalf("after update: %v %s", err, fragment.Status)
	}
	if got := len(repo.All()); got != 0 {
		t.Fatalf("draft should leave listings, got %d", got)
	}
	if err := repo.Schedule("my-post", time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if err := invisible.Load(); err != nil {
		t.Fatal(err)
	}
	if fragment, _ := invisible.BySlug("my-post"); fragment.Status != StatusScheduled {
		t.Fatalf("after schedule: %s", fragment.Status)
	}
	if err := repo.Archive("my-post"); err != nil {
		t.Fatal(err)
	}
	if err := invisible.Load(); err != nil {
		t.Fatal(err)
	}
	if fragment, _ := invisible.BySlug("my-post"); fragment.Status != StatusArchived {
		t.Fatalf("after archive: %s", fragment.Status)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "post.md"))
	if !strings.Contains(string(data), "status: archived") {
		t.Fatalf("front matter not rewritten: %s", data)
	}
}

func TestCustomURLBuilderAndDatedSorting(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "a.md", "---\ntitle: A\ndate: 2026-01-01\n---\nA body")
	writeContent(t, dir, "b.md", "---\ntitle: B\ndate: 2026-05-01\n---\nB body")
	repo := NewFileSystemRepository(RepositoryOptions{
		Path: dir,
		URLBuilder: func(f *Fragment) string {
			return "/blog/" + f.Date.Format("2006/01") + "/" + f.Slug
		},
		Now: fixedNow,
	})
	if err := repo.Load(); err != nil {
		t.Fatal(err)
	}
	all := repo.All()
	if len(all) != 2 || all[0].Title != "B" {
		t.Fatalf("newest-first ordering broken: %v", all)
	}
	if all[0].URL != "/blog/2026/05/b" {
		t.Fatalf("url builder not applied: %s", all[0].URL)
	}
	if fragment, err := repo.ByURL("/blog/2026/05/b"); err != nil || fragment.Title != "B" {
		t.Fatalf("by-url lookup failed: %v", err)
	}
}

func TestSanitizerProfiles(t *testing.T) {
	body := "Hello <script>alert(1)</script> [x](https://e.com) world"
	relaxed := NewMarkdownParser(SanitizerRelaxedTrustedAuthor).Render
	strict := NewMarkdownParser(SanitizerStrict).Render
	relaxedHTML, err := relaxed([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(relaxedHTML, "<script") {
		t.Fatalf("relaxed must strip scripts: %s", relaxedHTML)
	}
	if !strings.Contains(relaxedHTML, "https://e.com") {
		t.Fatalf("relaxed should keep links: %s", relaxedHTML)
	}
	strictHTML, err := strict([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strictHTML, "<a ") {
		t.Fatalf("strict must strip links: %s", strictHTML)
	}
}

func TestAuthorRepository(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "jane.author.yml", "name: Jane Doe\nbio: Writes things.\ngithub: janedoe\n")
	repo, err := NewAuthorRepository(dir)
	if err != nil {
		t.Fatal(err)
	}
	author := repo.Resolve("Jane Doe")
	if author == nil || author.Bio != "Writes things." {
		t.Fatalf("resolve by name failed: %+v", author)
	}
	if repo.Resolve("nobody") != nil || repo.Resolve("") != nil {
		t.Fatal("unknown/empty should resolve nil")
	}
	if author.Slug != "jane-doe" {
		t.Fatalf("slug = %s", author.Slug)
	}
	links := author.SocialEntries()
	if len(links) != 1 || links[0].URL != "https://github.com/janedoe" {
		t.Fatalf("social entries = %v", links)
	}
}

func TestReadingTimeOf(t *testing.T) {
	if ReadingTimeOf("") != 0 {
		t.Fatal("empty should be 0")
	}
	if got := ReadingTimeOf(strings.Repeat("word ", 600)); got != 3 {
		t.Fatalf("600 words = %d, want 3", got)
	}
	if got := ReadingTimeOf("one two three"); got != 1 {
		t.Fatalf("short = %d, want 1", got)
	}
}

func TestExcludeOption(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "keep.md", "---\ntitle: Keep\n---\nBody")
	writeContent(t, dir, "outerstellar-secret.md", "---\ntitle: Hidden\n---\nBody")
	repo := NewFileSystemRepository(RepositoryOptions{
		Path:    dir,
		Exclude: func(name string) bool { return strings.Contains(strings.ToLower(name), "outerstellar") },
	})
	if err := repo.Load(); err != nil {
		t.Fatal(err)
	}
	if got := len(repo.All()); got != 1 {
		t.Fatalf("all = %d, want 1 (excluded file dropped)", got)
	}
}

func TestVisibleFalseHides(t *testing.T) {
	dir := t.TempDir()
	writeContent(t, dir, "hidden.md", "---\ntitle: Hidden\nvisible: false\n---\nBody")
	writeContent(t, dir, "shown.md", "---\ntitle: Shown\n---\nBody")
	repo := NewFileSystemRepository(RepositoryOptions{Path: dir, Now: fixedNow})
	if err := repo.Load(); err != nil {
		t.Fatal(err)
	}
	if got := len(repo.All()); got != 1 {
		t.Fatalf("all = %d, want 1 (visible:false hidden)", got)
	}
}
