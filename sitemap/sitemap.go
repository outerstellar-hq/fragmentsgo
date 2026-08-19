// Package sitemap builds sitemap.xml documents from page URLs.
package sitemap

import (
	"encoding/xml"
	"strings"
)

// URLSet is the rendered sitemap document.
type URLSet struct {
	XMLName xml.Name `xml:"urlset"`
	XMLNS   string   `xml:"xmlns,attr"`
	URLs    []entry  `xml:"url"`
}

type entry struct {
	Loc string `xml:"loc"`
}

// Builder assembles a sitemap.
type Builder struct {
	siteURL string
	set     URLSet
}

// New creates a builder; siteURL prefixes every location.
func New(siteURL string) *Builder {
	return &Builder{siteURL: strings.TrimRight(siteURL, "/"), set: URLSet{XMLNS: "http://www.sitemaps.org/schemas/sitemap/0.9"}}
}

// Add appends a path (or absolute URL) to the sitemap.
func (b *Builder) Add(path string) *Builder {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		b.set.URLs = append(b.set.URLs, entry{Loc: path})
		return b
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	b.set.URLs = append(b.set.URLs, entry{Loc: b.siteURL + path})
	return b
}

// AddAll appends multiple paths.
func (b *Builder) AddAll(paths []string) *Builder {
	for _, path := range paths {
		b.Add(path)
	}
	return b
}

// Build returns the assembled URL set.
func (b *Builder) Build() *URLSet {
	return &b.set
}

// Bytes renders the sitemap including the XML header.
func (b *Builder) Bytes() ([]byte, error) {
	data, err := xml.MarshalIndent(&b.set, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), data...), nil
}
