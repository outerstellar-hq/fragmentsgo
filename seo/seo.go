// Package seo builds per-page SEO metadata: Open Graph and Twitter tags,
// canonical URLs, robots directives, and JSON-LD structured data — the Go
// counterpart of fragments4k's fragments-seo module.
package seo

import (
	"encoding/json"
	"html/template"
	"strings"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
)

// Robots values for the robots meta tag.
const (
	RobotsIndex         = ""
	RobotsNoIndex       = "noindex"
	RobotsNoIndexFollow = "noindex, follow"
)

// Site describes the consuming site for metadata generation.
type Site struct {
	Name        string
	URL         string
	Description string
	ImageURL    string // defaults to SiteURL + /logo.png
}

// Metadata is the SEO data for one page.
type Metadata struct {
	Title         string
	Description   string
	CanonicalURL  string
	Robots        string
	OGType        string // "website" or "article"
	PublishedTime string // ISO 8601, article types
	ImageURL      string
	SiteName      string
	JSONLD        []string
}

// ForFragment builds article-style metadata from a fragment.
func ForFragment(site Site, fragment *fragmentsgo.Fragment, ogType string) Metadata {
	meta := Metadata{
		Title:         fragment.Title,
		Description:   clamp(fragment.Preview, 300),
		CanonicalURL:  site.URL + fragment.URL,
		OGType:        ogType,
		PublishedTime: isoDate(fragment.Date),
		ImageURL:      imageFor(site, fragment.Image),
		SiteName:      site.Name,
	}
	return meta
}

// ForPage builds metadata for a listing or standalone page.
func ForPage(site Site, title, description, path, robots string) Metadata {
	return Metadata{
		Title:        title,
		Description:  clamp(description, 300),
		CanonicalURL: site.URL + path,
		Robots:       robots,
		OGType:       "website",
		ImageURL:     imageFor(site, ""),
		SiteName:     site.Name,
	}
}

// WithJSONLD appends serialized JSON-LD payloads.
func (m Metadata) WithJSONLD(payloads ...string) Metadata {
	m.JSONLD = append(m.JSONLD, payloads...)
	return m
}

// OrganizationJSONLD returns an Organization structured-data block.
func OrganizationJSONLD(site Site) string {
	return jsonLD(map[string]any{
		"@context": "https://schema.org",
		"@type":    "Organization",
		"name":     site.Name,
		"url":      site.URL,
		"logo":     imageFor(site, ""),
	})
}

// WebSiteJSONLD returns a WebSite block with a search action pointing at
// searchPath + "q={search_term_string}".
func WebSiteJSONLD(site Site, searchPath string) string {
	return jsonLD(map[string]any{
		"@context": "https://schema.org",
		"@type":    "WebSite",
		"name":     site.Name,
		"url":      site.URL,
		"potentialAction": map[string]any{
			"@type":       "SearchAction",
			"target":      site.URL + searchPath + "q={search_term_string}",
			"query-input": "required name=search_term_string",
		},
	})
}

// BreadcrumbJSONLD returns a BreadcrumbList for the given trail of
// (name, path) pairs.
func BreadcrumbJSONLD(site Site, trail [][2]string) string {
	items := make([]map[string]any, 0, len(trail))
	for index, entry := range trail {
		items = append(items, map[string]any{
			"@type":    "ListItem",
			"position": index + 1,
			"name":     entry[0],
			"item":     site.URL + entry[1],
		})
	}
	return jsonLD(map[string]any{
		"@context":        "https://schema.org",
		"@type":           "BreadcrumbList",
		"itemListElement": items,
	})
}

// BlogPostingJSONLD returns a BlogPosting block for a fragment.
func BlogPostingJSONLD(site Site, fragment *fragmentsgo.Fragment) string {
	return jsonLD(map[string]any{
		"@context":      "https://schema.org",
		"@type":         "BlogPosting",
		"headline":      fragment.Title,
		"description":   fragment.Preview,
		"datePublished": isoDate(fragment.Date),
		"url":           site.URL + fragment.URL,
		"author":        map[string]string{"@type": "Person", "name": fragment.Author},
		"publisher":     map[string]any{"@type": "Organization", "name": site.Name},
	})
}

// HeadTags renders the metadata as HTML tags for <head>: Open Graph,
// Twitter card, canonical, robots, and JSON-LD script blocks. Verification
// codes (google/bing) are emitted when non-empty.
func (m Metadata) HeadTags(googleVerification, bingVerification string) string {
	var builder strings.Builder
	esc := template.HTMLEscapeString
	builder.WriteString(`<meta property="og:site_name" content="` + esc(m.SiteName) + `">`)
	builder.WriteString(`<meta property="og:title" content="` + esc(m.Title) + `">`)
	builder.WriteString(`<meta property="og:description" content="` + esc(m.Description) + `">`)
	ogType := m.OGType
	if ogType == "" {
		ogType = "website"
	}
	builder.WriteString(`<meta property="og:type" content="` + ogType + `">`)
	builder.WriteString(`<meta property="og:url" content="` + esc(m.CanonicalURL) + `">`)
	if m.ImageURL != "" {
		builder.WriteString(`<meta property="og:image" content="` + esc(m.ImageURL) + `">`)
	}
	builder.WriteString(`<meta name="twitter:card" content="summary">`)
	builder.WriteString(`<link rel="canonical" href="` + esc(m.CanonicalURL) + `">`)
	if m.Robots != "" {
		builder.WriteString(`<meta name="robots" content="` + m.Robots + `">`)
	}
	if m.OGType == "article" && m.PublishedTime != "" {
		builder.WriteString(`<meta property="article:published_time" content="` + m.PublishedTime + `">`)
	}
	if googleVerification != "" {
		builder.WriteString(`<meta name="google-site-verification" content="` + esc(googleVerification) + `">`)
	}
	if bingVerification != "" {
		builder.WriteString(`<meta name="msvalidate.01" content="` + esc(bingVerification) + `">`)
	}
	for _, payload := range m.JSONLD {
		if payload == "" {
			continue
		}
		builder.WriteString(`<script type="application/ld+json">` + payload + `</script>`)
	}
	return builder.String()
}

func imageFor(site Site, fragmentImage string) string {
	if fragmentImage != "" {
		return fragmentImage
	}
	if site.ImageURL != "" {
		return site.ImageURL
	}
	return site.URL + "/logo.png"
}

func clamp(text string, limit int) string {
	if len(text) <= limit {
		return text
	}
	return text[:limit-1] + "…"
}

func isoDate(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format("2006-01-02T15:04:05Z07:00")
}

func jsonLD(payload any) string {
	data, err := json.Marshal(payload)
	if err != nil {
		return ""
	}
	return string(data)
}
