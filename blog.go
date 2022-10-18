package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

var blogURLPrefix = "/blog"
var blogHP = NewHostedPath("blog/", blogURLPrefix+"/static/")

// BlogVideo bundles static file URLs to all encodings of a video with all
// necessary metadata.
type BlogVideo struct {
	Metadata *VideoMetadata
	Poster   template.HTML
	Sources  []template.HTML
	Date     string
	ID       string
	Title    template.HTML
	Alt      string
	Active   bool
	NoLoop   bool
}

func (b *BlogVideo) SetID(id string) *BlogVideo {
	b.ID = id
	return b
}

func (b *BlogVideo) SetTitle(title template.HTML) string {
	b.Title = title
	return ""
}

func (b *BlogVideo) SetActive() *BlogVideo {
	b.Active = true
	return b
}

func (b *BlogVideo) SetNoLoop() *BlogVideo {
	b.NoLoop = true
	return b
}

// Tag generates a complete HTML <video> tag for a video.
func (b *BlogVideo) Tag() (ret template.HTML) {
	ret += (`<video preload="none" controls poster="` + b.Poster + `"`)
	if b.ID != "" {
		ret += template.HTML(fmt.Sprintf(` id="%s-%s"`, b.Date, b.ID))
	}
	if !b.NoLoop {
		ret += ` loop`
	}
	if b.Active {
		ret += ` class="active"`
	}
	ret += template.HTML(fmt.Sprintf(
		` data-fps="%v" data-frame-count="%v"`,
		b.Metadata.FPS, b.Metadata.FrameCount,
	))

	ret += `>`
	for _, source := range b.Sources {
		ret += `<source src="` + source + `" type="video/webm">`
	}

	if b.Alt != "" {
		ret += template.HTML(b.Alt + ". ")
	}
	ret += template.HTML(fmt.Sprintf(`<a href="%s">Download</a>`, b.Sources[0]))
	ret += `</video>`
	return ret
}

func (b *Blog) NewBlogVideo(stem, date, alt string) *BlogVideo {
	ret := &BlogVideo{
		Metadata: &b.Video.Cache.Video[stem].Metadata,
		Poster:   template.HTML(b.VideoURL(stem, &POSTER)),
		Date:     date,
		Alt:      alt,
	}
	for _, codec := range VIDEO_HOSTED {
		codecURL := template.HTML(b.VideoURL(stem, codec))
		ret.Sources = append(ret.Sources, codecURL)
	}
	return ret
}

// BlogEntry describes an existing blog entry, together with information about
// its associated pushes parsed from the database.
type BlogEntry struct {
	Date         string
	Pushes       []Push
	Tags         []string
	templateName string
}

// GetTime returns the timestamp corresponding to the moment the blog entry has
// been published. It matches the datetime string on the blog.
func (e *BlogEntry) GetTime() time.Time {
	if e.Pushes != nil {
		return e.Pushes[0].Delivered
	} else {
		return DateInDevLocation(e.Date).Time
	}
}

// Blog bundles all blog entries, sorted from newest to oldest.
type Blog struct {
	Entries []*BlogEntry
	Video   *VideoRoot
}

// NewBlog parses all HTML files in the blog path into t, and returns a new
// sorted Blog. funcs can be used to add any template functions that rely on
// a Blog instance.
func NewBlog(t *template.Template, pushes tPushes, tags tBlogTags, videoRoot *VideoRoot, funcs func(b *Blog) map[string]interface{}) *Blog {
	ret := &Blog{Video: videoRoot}
	// Unlike Go's own template.ParseGlob, we want to prefix template names
	// with their local path.
	templates, err := filepath.Glob(filepath.Join(blogHP.LocalPath, "*.html"))
	FatalIf(err)
	sort.Slice(templates, func(i, j int) bool { return templates[i] > templates[j] })
	for _, tmpl := range templates {
		basename := filepath.Base(tmpl)
		date := strings.TrimSuffix(basename, path.Ext(basename))
		ret.Entries = append(ret.Entries, &BlogEntry{
			Date:         date,
			Pushes:       pushes.DeliveredAt(date),
			Tags:         tags[date],
			templateName: tmpl,
		})
	}
	t.Funcs(funcs(ret))
	for _, tmpl := range templates {
		buf, err := os.ReadFile(tmpl)
		FatalIf(err)
		template.Must(t.New(tmpl).Parse(string(buf)))
	}
	return ret
}

// OldVideoRedirectHandler redirects old VP9 and VP8 video URLs to the new
// codec-specific subdirectories.
func (b *Blog) OldVideoRedirectHandler(vd *VideoDir) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		newURL := b.VideoURL(mux.Vars(req)["stem"], vd)
		http.Redirect(wr, req, newURL, http.StatusMovedPermanently)
	})
}

// FindEntryByString looks for and returns a potential blog entry posted
// during the given ISO 8601-formatted date, or nil if there is none.
func (b *Blog) FindEntryByString(date string) (*BlogEntry, error) {
	// Note that we don't use sort.SearchStrings() here, since we're sorted
	// in descending order!
	i := sort.Search(len(b.Entries), func(i int) bool {
		return b.Entries[i].Date <= date
	})
	if i >= len(b.Entries) || b.Entries[i].Date != date {
		return nil, eNoPost{date}
	}
	return b.Entries[i], nil
}

// FindEntryByTime looks for and returns a potential blog entry posted during
// the date of the given Time instance, or nil if there is none.
func (b *Blog) FindEntryByTime(date time.Time) *BlogEntry {
	entry, _ := b.FindEntryByString(date.Format("2006-01-02"))
	return entry
}

// FindEntryForPush looks for and returns a potential blog entry which
// summarizes the given Push.
func (b *Blog) FindEntryForPush(p Push) *BlogEntry {
	return b.FindEntryByTime(p.Delivered)
}

// PostDot contains everything handed to a blog template as the value of dot.
type PostDot struct {
	Date       string      // ISO 8601-formatted date
	HostedPath *HostedPath // Value of [blogHP]
	DatePrefix string      // Date prefix for potential post-specific files
	// Generates [HostedPath.URLPrefix] + [DatePrefix]
	PostFileURL func(fn string) template.HTML
	Video       func(fn string, alt string) *BlogVideo
	VideoPlayer func(videos ...*BlogVideo) template.HTML
}

// Post bundles the rendered HTML body of a post with all necessary header
// data.
type Post struct {
	Date     string
	Time     time.Time // Full post time
	PushIDs  []ScopedID
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

// VideoURL returns the URL of a video in a specific codec.
func (b *Blog) VideoURL(stem string, vd *VideoDir) string {
	return blogHP.VersionURLFor(b.Video.URL(stem, vd))
}

// Render builds a new Post instance from e.
func (b *Blog) Render(e *BlogEntry, filters []string) Post {
	var builder strings.Builder
	datePrefix := e.Date + "-"
	ctx := PostDot{
		Date:       e.Date,
		HostedPath: blogHP,
		DatePrefix: datePrefix,
		PostFileURL: func(fn string) template.HTML {
			return template.HTML(blogHP.VersionURLFor(datePrefix + fn))
		},
		Video: func(fn string, alt string) *BlogVideo {
			return b.NewBlogVideo((datePrefix + fn), e.Date, alt)
		},
		VideoPlayer: func(videos ...*BlogVideo) (ret template.HTML) {
			ret = "<rec98-video>"
			for _, video := range videos {
				ret += video.Tag()
			}
			ret += `<rec98-parent-init></rec98-parent-init>`
			ret += `</rec98-video>`
			return ret
		},
	}
	pagesExecute(&builder, e.templateName, &ctx)

	post := Post{
		Date:    e.Date,
		Time:    e.GetTime(),
		Tags:    e.Tags,
		Filters: filters,
		Body:    template.HTML(builder.String()),
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
func (b *Blog) GetPost(date string) (*Post, error) {
	entry, err := b.FindEntryByString(date)
	if err != nil {
		return nil, err
	}
	post := b.Render(entry, []string{})
	return &post, nil
}

// Posts renders all blog posts that match the given slice of filters. Pass an
// empty slice to get all posts.
func (b *Blog) Posts(filters []string) chan Post {
	ret := make(chan Post)
	go func() {
		for _, entry := range b.Entries {
			filtersSeen := 0
			for _, tag := range entry.Tags {
				for _, filter := range filters {
					if filter == tag {
						filtersSeen++
					}
				}
			}
			if filtersSeen == len(filters) {
				ret <- b.Render(entry, filters)
			}
		}
		close(ret)
	}()
	return ret
}

// PostLink returns a nicely formatted link to the given blog post.
func (b *Blog) PostLink(date string, text string) template.HTML {
	_, err := b.FindEntryByString(date)
	FatalIf(err)
	return template.HTML(fmt.Sprintf(
		`<a href="%s/%s">📝 %s</a>`, blogURLPrefix, date, text,
	))
}
