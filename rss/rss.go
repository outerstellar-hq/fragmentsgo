// Package rss builds RSS 2.0 feeds from fragments.
package rss

import (
	"bytes"
	"encoding/xml"
	"io"
	"strings"
	"time"

	"github.com/outerstellar-hq/fragmentsgo"
)

// Channel describes the feed.
type Channel struct {
	Title       string
	Link        string
	Description string
}

// Feed is a rendered RSS 2.0 document.
type Feed struct {
	XMLName xml.Name `xml:"rss"`
	Version string   `xml:"version,attr"`
	Channel channel  `xml:"channel"`
}

type channel struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	Items       []item `xml:"item"`
}

type item struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	GUID        string `xml:"guid"`
}

// Build creates a feed from the fragments in listing order.
func Build(content Channel, fragments []*fragmentsgo.Fragment) *Feed {
	feed := &Feed{
		Version: "2.0",
		Channel: channel{Title: content.Title, Link: content.Link, Description: content.Description},
	}
	for _, fragment := range fragments {
		entry := item{
			Title:       fragment.Title,
			Link:        absoluteLink(content.Link, fragment),
			Description: fragment.Preview,
			GUID:        absoluteLink(content.Link, fragment),
		}
		if !fragment.Date.IsZero() {
			entry.PubDate = fragment.Date.Format(time.RFC1123Z)
		}
		feed.Channel.Items = append(feed.Channel.Items, entry)
	}
	return feed
}

// Write renders the feed to w including the XML header.
func (f *Feed) Write(w io.Writer) error {
	if _, err := w.Write([]byte(xml.Header)); err != nil {
		return err
	}
	return xml.NewEncoder(w).Encode(f)
}

// Bytes renders the feed as an XML byte slice.
func (f *Feed) Bytes() ([]byte, error) {
	var buffer bytes.Buffer
	if err := f.Write(&buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// absoluteLink resolves a fragment URL against the channel link's origin
// (scheme + host): fragment URLs already carry their full path, so
// "https://demo.test/blog" + "/blog/2026/01/post" must not double the
// /blog prefix.
func absoluteLink(base string, fragment *fragmentsgo.Fragment) string {
	if !strings.HasPrefix(fragment.URL, "/") {
		return fragment.URL
	}
	origin := base
	if scheme := strings.Index(base, "://"); scheme > 0 {
		if slash := strings.Index(base[scheme+3:], "/"); slash >= 0 {
			origin = base[:scheme+3+slash]
		}
	}
	return strings.TrimRight(origin, "/") + fragment.URL
}
