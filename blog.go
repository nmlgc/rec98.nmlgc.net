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

// HasEntryFor returns the ID of a potential blog entry posted at the given
// date, or nil if there is none.
func (b Blog) HasEntryFor(date time.Time) *string {
	ds := date.Format("2006-01-02")
	filename := filepath.Join(blogHP.LocalPath, ds+".html")
	// Note that we don't use sort.SearchStrings() here, since we're sorted
	// in descending order!
	i := sort.Search(len(b), func(i int) bool { return b[i] <= filename })
	// i := sort.SearchStrings(b, filename)
	if i >= len(b) || b[i] != filename {
		return nil
	}
	return &ds
}

// PostDot contains everything handed to a blog template as the value of dot.
type PostDot struct {
	PostPrefix template.HTML // Prefix for potential post-specific files
}

// Post bundles the rendered HTML body of a post, together with information
// about its associated pushes, parsed from the database.
type Post struct {
	Date     string
	Time     time.Time // Full post time
	PushIDs  []string
	FundedBy []CustomerID
	Diffs    []DiffInfo
	Body     template.HTML
}

// Posts renders all blog posts.
func Posts() chan Post {
	ret := make(chan Post)
	go func() {
		for _, tmplName := range blog {
			var b strings.Builder

			basename := filepath.Base(tmplName)
			date := strings.TrimSuffix(basename, path.Ext(basename))
			ctx := PostDot{
				PostPrefix: template.HTML(blogHP.URLPrefix + date + "-"),
			}
			pagesExecute(&b, tmplName, &ctx)

			pushes := PushesDeliveredAt(date)
			post := Post{
				Date: date,
				Time: *pushes[0].Delivered.Time,
				Body: template.HTML(b.String()),
			}

			for i := len(pushes) - 1; i >= 0; i-- {
				push := &pushes[i]
				post.PushIDs = append(post.PushIDs, push.ID)
				post.Diffs = append(post.Diffs, NewDiffInfo(push.Diff))
				post.FundedBy = append(post.FundedBy, push.Customer)
			}
			RemoveDuplicates(&post.Diffs)
			RemoveDuplicates(&post.FundedBy)
			ret <- post
		}
		close(ret)
	}()
	return ret
}
