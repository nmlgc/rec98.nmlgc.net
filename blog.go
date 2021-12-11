package main

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var blogURLPrefix = "/blog"
var blogHP = newHostedPath("blog/", blogURLPrefix+"/static/")

// BlogEntry describes an existing blog entry, together with information about
// its associated pushes parsed from the database.
type BlogEntry struct {
	Date         string
	Pushes       []Push
	Tags         []string
	templateName string
}

// Blog bundles all blog entries, sorted from newest to oldest.
type Blog []BlogEntry

// NewBlog parses all HTML files in the blog path into t, and returns a new
// sorted Blog.
func NewBlog(t *template.Template, pushes tPushes, tags tBlogTags) (ret Blog) {
	// Unlike Go's own template.ParseGlob, we want to prefix template names
	// with their local path.
	templates, err := filepath.Glob(filepath.Join(blogHP.LocalPath, "*.html"))
	FatalIf(err)
	sort.Slice(templates, func(i, j int) bool { return templates[i] > templates[j] })
	for _, tmpl := range templates {
		basename := filepath.Base(tmpl)
		date := strings.TrimSuffix(basename, path.Ext(basename))
		ret = append(ret, BlogEntry{
			Date:         date,
			Pushes:       pushes.DeliveredAt(date),
			Tags:         tags[date],
			templateName: tmpl,
		})
	}
	for _, tmpl := range templates {
		buf, err := ioutil.ReadFile(tmpl)
		FatalIf(err)
		template.Must(t.New(tmpl).Parse(string(buf)))
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
	HostedPath *hostedPath // Value of [blogHP]
	DatePrefix string      // Date prefix for potential post-specific files
	// Generates [HostedPath.URLPrefix] + [DatePrefix]
	PostFileURL func(fn string) template.HTML
}

// Post bundles the rendered HTML body of a post with all necessary header
// data.
type Post struct {
	Date     string
	Time     time.Time // Full post time
	PushIDs  []PushID
	FundedBy []CustomerID
	Diffs    []DiffInfo
	Tags     []string
	Filters  []string
	Body     template.HTML
}

type eNoPost struct {
	date string
}

func (e eNoPost) Error() string {
	return fmt.Sprintf("no blog entry posted on %s", e.date)
}

// Render builds a new Post instance from e.
func (e BlogEntry) Render(filters []string) Post {
	var b strings.Builder
	datePrefix := e.Date + "-"
	ctx := PostDot{
		HostedPath: blogHP,
		DatePrefix: datePrefix,
		PostFileURL: func(fn string) template.HTML {
			return template.HTML(blogHP.VersionURLFor(datePrefix + fn))
		},
	}
	pagesExecute(&b, e.templateName, &ctx)

	post := Post{
		Date:    e.Date,
		Tags:    e.Tags,
		Filters: filters,
		Body:    template.HTML(b.String()),
	}
	if e.Pushes != nil {
		post.Time = e.Pushes[0].Delivered
	} else {
		post.Time = DateInDevLocation(e.Date).Time
	}

	for i := len(e.Pushes) - 1; i >= 0; i-- {
		push := &e.Pushes[i]
		post.PushIDs = append(post.PushIDs, push.ID)
		post.Diffs = append(post.Diffs, push.Diff)
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
	post := entry.Render([]string{})
	return &post, nil
}

// Posts renders all blog posts that match the given slice of filters. Pass an
// empty slice to get all posts.
func (b Blog) Posts(filters []string) chan Post {
	ret := make(chan Post)
	go func() {
		for _, entry := range b {
			filtersSeen := 0
			for _, tag := range entry.Tags {
				for _, filter := range filters {
					if filter == tag {
						filtersSeen++
					}
				}
			}
			if filtersSeen == len(filters) {
				ret <- entry.Render(filters)
			}
		}
		close(ret)
	}()
	return ret
}

// PostLink returns a nicely formatted link to the given blog post.
func PostLink(date string, text string) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<a href="%s/%s">📝 %s</a>`, blogURLPrefix, date, text,
	))
}
