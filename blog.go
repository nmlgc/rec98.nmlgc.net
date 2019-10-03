package main

import (
	"html/template"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var blogHP = newHostedPath("blog/", "/blog/")

// Blog contains the names of all blog post templates, sorted from newest to
// oldest.
type Blog []string

var blog = func() Blog {
	ret := pagesParseSubdirectory(blogHP.LocalPath, "*.html")
	sort.Slice(ret, func(i, j int) bool { return ret[i] > ret[j] })
	return Blog(ret)
}()

// BlogEntry identifies an existing blog entry.
type BlogEntry struct {
	Date         string
	templateName string
}

// HasEntryFor returns the ID of a potential blog entry posted at the given
// date, or nil if there is none.
func (b Blog) HasEntryFor(date time.Time) *BlogEntry {
	ds := date.Format("2006-01-02")
	filename := filepath.Join(blogHP.LocalPath, ds+".html")
	// Note that we don't use sort.SearchStrings() here, since we're sorted
	// in descending order!
	i := sort.Search(len(b), func(i int) bool { return b[i] <= filename })
	// i := sort.SearchStrings(b, filename)
	if i >= len(b) || b[i] != filename {
		return nil
	}
	return &BlogEntry{
		Date:         ds,
		templateName: filename,
	}
}

// PostDot contains everything handed to a blog template as the value of dot.
type PostDot struct {
	FilePrefix template.HTML // Prefix for potential post-specific files
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

// Render builds a new Post instance from e.
func (e BlogEntry) Render() Post {
	var b strings.Builder
	ctx := PostDot{
		FilePrefix: template.HTML(blogHP.URLPrefix + e.Date + "-"),
	}
	pagesExecute(&b, e.templateName, &ctx)

	pushes := PushesDeliveredAt(e.Date)
	post := Post{
		Date: e.Date,
		Time: pushes[0].Delivered,
		Body: template.HTML(b.String()),
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

// Posts renders all blog posts.
func Posts() chan Post {
	ret := make(chan Post)
	go func() {
		for _, tmpl := range blog {
			basename := filepath.Base(tmpl)
			date := strings.TrimSuffix(basename, path.Ext(basename))
			ret <- BlogEntry{
				Date:         date,
				templateName: tmpl,
			}.Render()
		}
		close(ret)
	}()
	return ret
}
