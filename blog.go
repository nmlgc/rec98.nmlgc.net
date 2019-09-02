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

var blogPosts = func() []string {
	ret := pagesParseSubdirectory(blogHP.LocalPath, "*.html")
	sort.Slice(ret, func(i, j int) bool { return ret[i] > ret[j] })
	return ret
}()

// DiffInfo contains all pieces of information parsed from a GitHub diff URL.
type DiffInfo struct {
	URL     string
	Project string
	Rev     string
}

// NewDiffInfo parses a GitHub diff URL into a DiffInfo structure.
func NewDiffInfo(url string) DiffInfo {
	s := strings.Split(url, "/")
	project := ""
	if s[1] == "rec98.nmlgc.net" {
		project = "Website"
	}
	return DiffInfo{
		URL:     url,
		Project: project,
		Rev:     s[len(s)-1],
	}
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
		for _, tmplName := range blogPosts {
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
