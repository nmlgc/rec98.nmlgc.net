package main

import (
	"html/template"
	"path"
	"path/filepath"
	"strings"
)

var blogHP = newHostedPath("blog/", "/blog/")

var blogPosts = pagesParseSubdirectory(blogHP.LocalPath, "*.html")

// PostDot contains everything handed to a blog template as the value of dot.
type PostDot struct {
	PostPrefix template.HTML // Prefix for potential post-specific files
}

// Posts renders all blog posts.
func Posts() chan template.HTML {
	ret := make(chan template.HTML)
	go func() {
		for _, tmplName := range blogPosts {
			var b strings.Builder

			basename := filepath.Base(tmplName)
			date := strings.TrimSuffix(basename, path.Ext(basename))
			ctx := PostDot{
				PostPrefix: template.HTML(blogHP.URLPrefix + date + "-"),
			}
			pagesExecute(&b, tmplName, &ctx)
			ret <- template.HTML(b.String())
		}
		close(ret)
	}()
	return ret
}
