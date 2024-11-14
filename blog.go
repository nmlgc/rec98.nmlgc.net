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

// BlogVideoMarker specifies an interesting frame in the video that should be
// highlighted on its timeline.
type BlogVideoMarker struct {
	Frame     uint
	Title     string
	Alignment template.HTML
}

type BlogPlayerElement interface {
	// Tag generates a complete HTML tag for a single element.
	Tag() (ret template.HTML)
}

// BlogPlayerElementMeta bundles all shared metadata for an element shown in
// a <rec98-player> subclass.
type BlogPlayerElementMeta struct {
	Date   string
	Title  template.HTML
	Alt    string
	Active bool
	NoLoop bool
}

func (m *BlogPlayerElementMeta) tagAttributes() (ret template.HTML) {
	ret += ` preload="none" controls`
	if m.Title != "" {
		ret += template.HTML(fmt.Sprintf(` data-title="%s"`, m.Title))
	}
	if !m.NoLoop {
		ret += ` loop`
	}
	if m.Active {
		ret += ` data-active`
	}
	return
}

func (m *BlogPlayerElementMeta) tagBody(lossless template.HTML) (ret template.HTML) {
	if m.Alt != "" {
		ret += template.HTML(m.Alt + ". ")
	}
	ret += template.HTML(fmt.Sprintf(`<a href="%s">Download</a>`, lossless))
	return ret
}

func blogPlayerBody[T BlogPlayerElement](elements []T) (ret template.HTML) {
	for _, element := range elements {
		ret += element.Tag()
	}
	ret += `<rec98-parent-init></rec98-parent-init>`
	return
}

// BlogVideo bundles static file URLs to all encodings of a video with all
// necessary metadata.
type BlogVideo struct {
	BlogPlayerElementMeta
	Metadata *VideoMetadata
	Poster   template.HTML
	Lossless template.HTML
	Sources  []template.HTML
	Markers  []BlogVideoMarker
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

func (b *BlogVideo) AddMarker(frame uint, title string, alignment template.HTML) string {
	b.Markers = append(b.Markers, BlogVideoMarker{
		Frame: frame, Title: title, Alignment: alignment,
	})
	return ""
}

func (b *BlogVideo) LinkMarkers(other *BlogVideo) string {
	b.Markers = other.Markers
	return ""
}

// FigureAttrs generates attributes for the <figure> tag that contains the
// given video.
func (b *BlogVideo) FigureAttrs() (ret template.HTMLAttr) {
	ret += template.HTMLAttr(
		fmt.Sprintf(`style="width: %dpx"`, b.Metadata.Width),
	)
	return ret
}

func (b *BlogVideo) Tag() (ret template.HTML) {
	ret += (`<video poster="` + b.Poster + `"`)
	ret += b.tagAttributes()
	ret += template.HTML(fmt.Sprintf(
		` width="%v" height="%v" data-fps="%v" data-frame-count="%v" style="aspect-ratio: %[1]d / %[2]d"`,
		b.Metadata.Width, b.Metadata.Height,
		b.Metadata.FPS, b.Metadata.FrameCount,
	))
	if b.Metadata.HasAudio {
		ret += ` data-audio`
	}
	ret += (` data-lossless="` + b.Lossless + `"`)

	ret += `>`
	for _, source := range b.Sources {
		ret += `<source src="` + source + `" type="video/webm">`
	}

	ret += b.tagBody(b.Lossless)
	for _, marker := range b.Markers {
		if marker.Alignment == "" {
			marker.Alignment = "right"
		}
		ret += "<rec98-video-marker"
		ret += template.HTML(fmt.Sprintf(
			` data-frame="%d" data-title="%s"`, marker.Frame, marker.Title,
		))
		ret += (` data-alignment="` + marker.Alignment + `"`)
		ret += "></rec98-video-marker>"
	}
	ret += `</video>`
	return ret
}

func (b *Blog) NewBlogVideo(stem, date, alt string) *BlogVideo {
	ret := &BlogVideo{
		BlogPlayerElementMeta: BlogPlayerElementMeta{
			Date: date,
			Alt:  alt,
		},
		Metadata: &b.Video.Cache.Video[stem].Metadata,
		Poster:   template.HTML(*b.VideoURL(stem, &POSTER)),
		Lossless: template.HTML(*b.VideoURL(stem, &VIDEO_SOURCE)),
	}
	for _, codec := range VIDEO_HOSTED {
		if codecURL := b.VideoURL(stem, codec); codecURL != nil {
			ret.Sources = append(ret.Sources, template.HTML(*codecURL))
		}
	}
	return ret
}

type BlogAudio struct {
	BlogPlayerElementMeta
	FLAC     template.HTML
	Waveform template.HTML
}

func (b *BlogAudio) SetTitle(title template.HTML) string {
	b.Title = title
	return ""
}

func (b *BlogAudio) SetActive() *BlogAudio {
	b.Active = true
	return b
}

func (b *BlogAudio) SetNoLoop() *BlogAudio {
	b.NoLoop = true
	return b
}

func (b *BlogAudio) Tag() (ret template.HTML) {
	ret += (`<audio src="` + b.FLAC + `" data-waveform="` + b.Waveform + `"`)
	ret += b.tagAttributes()
	ret += `>`
	ret += `</audio>`
	return ret
}

func (b *Blog) NewBlogAudio(stem, date, alt string) *BlogAudio {
	base := path.Join("audio", stem)
	return &BlogAudio{
		BlogPlayerElementMeta: BlogPlayerElementMeta{
			Date: date,
			Alt:  alt,
		},
		FLAC:     template.HTML(blogHP.VersionURLFor(base + ".flac")),
		Waveform: template.HTML(blogHP.VersionURLFor(base + ".png")),
	}
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
		if newURL == nil {
			wr.WriteHeader(http.StatusNotFound)
			return
		}
		http.Redirect(wr, req, *newURL, http.StatusMovedPermanently)
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
	Audio       func(fn string, alt string) *BlogAudio
	VideoPlayer func(videos ...*BlogVideo) template.HTML
	AudioPlayer func(videos ...*BlogAudio) template.HTML
}

// Post bundles the rendered HTML body of a post with all necessary header
// data.
type Post struct {
	Date     string
	Time     time.Time // Full post time
	Pushes   []*Push
	FundedBy []CustomerID
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

// VideoURL returns the URL of a video in a specific codec if it exists on the
// filesystem.
func (b *Blog) VideoURL(stem string, vd *VideoDir) *string {
	url := b.Video.URL(stem, vd)
	if url == nil {
		return nil
	}
	ret := blogHP.VersionURLFor(*url)
	return &ret
}

// PostHeader gathers all header info for e, leaving the post body empty.
func (b *Blog) PostHeader(e *BlogEntry, filters []string) *Post {
	ret := Post{
		Date:    e.Date,
		Time:    e.GetTime(),
		Tags:    e.Tags,
		Filters: filters,
	}
	for i := len(e.Pushes) - 1; i >= 0; i-- {
		push := &e.Pushes[i]
		ret.Pushes = append(ret.Pushes, push)
		ret.FundedBy = append(ret.FundedBy, push.FundedBy()...)
	}
	RemoveDuplicates(&ret.FundedBy)
	return &ret
}

// Render builds a new Post instance from e.
func (b *Blog) Render(e *BlogEntry, filters []string) *Post {
	var builder strings.Builder
	datePrefix := e.Date + "-"
	post := b.PostHeader(e, filters)
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
		Audio: func(fn string, alt string) *BlogAudio {
			return b.NewBlogAudio((datePrefix + fn), e.Date, alt)
		},
		VideoPlayer: func(videos ...*BlogVideo) (ret template.HTML) {
			ret = `<rec98-video class="rec98-player`
			for _, video := range videos {
				if len(video.Markers) > 0 {
					ret += ` with-markers`
					break
				}
			}
			ret += `">`
			ret += blogPlayerBody(videos)
			ret += `</rec98-video>`
			return ret
		},
		AudioPlayer: func(videos ...*BlogAudio) (ret template.HTML) {
			ret += `<rec98-audio class="rec98-player">`
			ret += blogPlayerBody(videos)
			ret += `</rec98-audio>`
			return ret
		},
	}
	pagesExecute(&builder, e.templateName, &ctx)
	post.Body = template.HTML(builder.String())
	return post
}

// GetPost returns the post that was originally posted on the given date.
func (b *Blog) GetPost(date string) (*Post, error) {
	entry, err := b.FindEntryByString(date)
	if err != nil {
		return nil, err
	}
	post := b.Render(entry, []string{})
	return post, nil
}

// Posts renders all blog posts that match the given slice of filters. Pass an
// empty slice to get all posts.
func (b *Blog) Posts(filters []string) chan *Post {
	ret := make(chan *Post)
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
func (b *Blog) PostLink(dateAndAnchor string, text string) template.HTML {
	date := dateAndAnchor
	anchor := ""
	if index := strings.LastIndexByte(dateAndAnchor, '#'); index != -1 {
		date = dateAndAnchor[:index]
		anchor = (dateAndAnchor[index:] + "-" + date)
	}
	_, err := b.FindEntryByString(date)
	FatalIf(err)
	return template.HTML(fmt.Sprintf(
		`<a href="%s/%s%s">📝 %s</a>`, blogURLPrefix, date, anchor, text,
	))
}
