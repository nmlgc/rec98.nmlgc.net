package main

import (
	"html/template"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/feeds"
)

// stripArticleRegexp matches <article id="{{.Date}}"> opening tags and
// </article> closing tags. They're stripped from the output because they're
// commonly blacklisted and it doesn't make much sense to have them in a feed.
var stripArticleRegexp = regexp.MustCompile(`(<article id="[0-9-]+">|</article>)`)

// FeedHandler provides net/http handlers for blog feed generation. It relies
// on blog entries being sorted from the newest to the oldest.
type FeedHandler struct {
	Blog     *Blog
	SiteURL  string
	BlogPath string
}

// getModTime returns the last modification time of the blog.
func (h *FeedHandler) getModTime() time.Time {
	if len(h.Blog.Entries) == 0 {
		return time.Time{}
	}
	return h.Blog.Entries[0].GetTime()
}

// getFeed generates a feed from the rendered blog entries.
func (h *FeedHandler) getFeed() *feeds.Feed {
	feed := &feeds.Feed{
		Title:       "ReC98",
		Link:        &feeds.Link{Href: h.SiteURL + h.BlogPath},
		Description: "The Touhou PC-98 Restoration Project",
		Updated:     h.getModTime(),
	}
	for post := range h.Blog.Posts(h.Blog.Render, nil) {
		var b strings.Builder
		pagesExecute(&b, "blog_post.html", &post)
		content := b.String()
		// NOTE(handlerug): Yes, I *will* use regular expressions to
		// parse HTML :zunpet:
		content = stripArticleRegexp.ReplaceAllString(content, "")
		link := (h.SiteURL + post.URL(""))
		feed.Add(&feeds.Item{
			Id:      link,
			Link:    &feeds.Link{Href: link},
			Created: post.Time,
			Content: content,
		})
	}
	return feed
}

// processRequest checks the HTTP method and handles conditional requests. It
// checks the If-Modified-Since header, and sets the Last-Modified header to
// the last blog entry's date. The return value indicates whether the request
// needs to be handled further.
func (h *FeedHandler) processRequest(wr http.ResponseWriter, req *http.Request) bool {
	if req.Method != "GET" && req.Method != "HEAD" {
		wr.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}

	// If there are no blog posts, we don't have anything to compare the
	// timestamps to. Generate the feeds unconditionally.
	if len(h.Blog.Entries) == 0 {
		return true
	}

	modTime := h.getModTime()
	wr.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))
	t, err := http.ParseTime(req.Header.Get("If-Modified-Since"))
	if err != nil {
		return true
	}
	// The Last-Modified header has sub-second precision truncated (due to
	// the http.TimeFormat having that omitted), so truncate modTime too.
	modTime = modTime.Truncate(time.Second)
	if modTime.Before(t) || modTime.Equal(t) {
		wr.WriteHeader(http.StatusNotModified)
		return false
	}
	return true
}

// FeedFormat specifies a feed format for both users and internally.
type FeedFormat struct {
	Name        string
	ContentType template.HTMLAttr
	Path        string // On the server, starting with a slash
	Handler     http.Handler
}

// newFormat instantiates an HTTP handler for a specific feed format and
// returns a FeedFormat instance to fully describe it.
func (h *FeedHandler) newFormat(name string, contentType string, basename string, writer func(f *feeds.Feed, wr io.Writer) error) *FeedFormat {
	return &FeedFormat{
		// HTMLAttr needs both key and value to fully prevent escaping.
		ContentType: template.HTMLAttr(`type="` + contentType + `"`),

		Name: name,
		Path: (h.BlogPath + "/" + basename),
		Handler: http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			if !h.processRequest(wr, req) {
				return
			}
			wr.Header().Set("Content-Type", contentType)
			writer(h.getFeed(), wr)
		}),
	}
}

// Formats returns feed handlers for all supported formats.
func (h *FeedHandler) Formats() []*FeedFormat {
	return []*FeedFormat{
		h.newFormat("RSS", "application/rss+xml", "feed.xml", (*feeds.Feed).WriteRss),
		h.newFormat("Atom", "application/atom+xml", "feed.atom", (*feeds.Feed).WriteAtom),

		// The content type for the version 1.1 of JSON feeds is
		// application/feed+json, by the way.
		h.newFormat("JSON", "application/json", "feed.json", (*feeds.Feed).WriteJSON),
	}
}
