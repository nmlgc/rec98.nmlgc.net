package main

import (
	"fmt"
	"html/template"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var blogURLPrefix = "/blog"
var blogHP = newHostedPath("blog/", blogURLPrefix+"/static/")

// BlogEntry identifies an existing blog entry.
type BlogEntry struct {
	Date         string
	templateName string
}

// Blog bundles all blog entires, sorted from newest to oldest.
type Blog []BlogEntry

// NewBlog parses all HTML files in the blog path into t, and returns a new
// sorted Blog.
func NewBlog(t *template.Template) (ret Blog) {
	templates := ParseSubdirectory(t, blogHP.LocalPath, "*.html")
	sort.Slice(templates, func(i, j int) bool { return templates[i] > templates[j] })
	for _, tmpl := range templates {
		basename := filepath.Base(tmpl)
		date := strings.TrimSuffix(basename, path.Ext(basename))
		ret = append(ret, BlogEntry{
			Date:         date,
			templateName: tmpl,
		})
	}
	return
}

// FindEntryByString looks for and returns a potential blog entry posted
// during the given ISO 8601-formatted date, or nil if there is none.
func (b Blog) FindEntryByString(date string) *BlogEntry {
	// Note that we don't use sort.SearchStrings() here, since we're sorted
	// in descending order!
	i := sort.Search(len(b), func(i int) bool { return b[i].Date <= date })
	if i >= len(b) || b[i].Date != date {
		return nil
	}
	return &b[i]
}

// FindEntryByTime looks for and returns a potential blog entry posted during
// the date of the given Time instance, or nil if there is none.
func (b Blog) FindEntryByTime(date time.Time) *BlogEntry {
	return b.FindEntryByString(date.Format("2006-01-02"))
}

// FindEntryForPush looks for and returns a potential blog entry which
// summarizes the given Push.
func (b Blog) FindEntryForPush(p Push) *BlogEntry {
	return b.FindEntryByTime(p.Delivered)
}

// PostDot contains everything handed to a blog template as the value of dot.
type PostDot struct {
	HostedPath hostedPath    // Value of [blogHP]
	DatePrefix string        // Date prefix for potential post-specific files
	FilePrefix template.HTML // [HostedPath.URLPrefix] + [DatePrefix]
}

// Post bundles the rendered HTML body of a post, together with information
// about its associated pushes, parsed from the database.
type Post struct {
	Date     string
	Time     time.Time // Full post time
	PushIDs  []PushID
	FundedBy []CustomerID
	Diffs    []DiffInfo
	Body     template.HTML
}

type eNoPost struct {
	date string
}

func (e eNoPost) Error() string {
	return fmt.Sprintf("no blog entry posted on %s", e.date)
}

// Render builds a new Post instance from e.
func (e BlogEntry) Render() Post {
	var b strings.Builder
	datePrefix := e.Date + "-"
	ctx := PostDot{
		HostedPath: blogHP,
		DatePrefix: datePrefix,
		FilePrefix: template.HTML(blogHP.URLPrefix + datePrefix),
	}
	pagesExecute(&b, e.templateName, &ctx)

	pushes := pushes.DeliveredAt(e.Date)
	post := Post{
		Date: e.Date,
		Body: template.HTML(b.String()),
	}
	if pushes != nil {
		post.Time = pushes[0].Delivered
	} else {
		post.Time = DateInDevLocation(e.Date).Time
	}

	for i := len(pushes) - 1; i >= 0; i-- {
		push := &pushes[i]
		post.PushIDs = append(post.PushIDs, push.ID)
		post.Diffs = append(post.Diffs, *push.Diff)
		post.FundedBy = append(post.FundedBy, push.FundedBy()...)
	}
	RemoveDuplicates(&post.Diffs)
	RemoveDuplicates(&post.FundedBy)
	return post
}

// GetPost returns the post that was originally posted on the given date.
func (b Blog) GetPost(date string) (*Post, error) {
	entry := b.FindEntryByString(date)
	if entry == nil {
		return nil, eNoPost{date}
	}
	post := entry.Render()
	return &post, nil
}

// Posts renders all blog posts.
func (b Blog) Posts() chan Post {
	ret := make(chan Post)
	go func() {
		for _, entry := range b {
			ret <- entry.Render()
		}
		close(ret)
	}()
	return ret
}

// PostLink returns a nicely formatted link to the given blog post.
func PostLink(date string, text string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<a href="%s/%s">📝 %s</code></a>`, blogURLPrefix, date, text,
	))
}
