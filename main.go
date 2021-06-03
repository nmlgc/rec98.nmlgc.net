package main

import (
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"math"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gorilla/mux"
)

// FatalIf removes the boilerplate for cases where errors are fatal.
func FatalIf(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

// hostedPath stores the corresponding URL prefix for a local filesystem path.
type hostedPath struct {
	srv http.Handler

	LocalPath string // must end with a slash
	URLPrefix string // must start *and* end with a slash
}

// newHostedPath sets up a new hostedPath instance.
func newHostedPath(LocalPath string, URLPrefix string) (ret hostedPath) {
	ret.LocalPath = path.Clean(LocalPath) + "/"
	ret.URLPrefix = URLPrefix
	dir := http.Dir(ret.LocalPath)
	ret.srv = http.FileServer(dir)
	return
}

// Server returns hp's file serving handler, e.g. to be re-used elsewhere.
func (hp hostedPath) Server() http.Handler {
	return hp.srv
}

// RegisterFileServer registers a HTTP route on the given router at hp's
// URLPrefix, serving any local files in hp's LocalPath.
func (hp hostedPath) RegisterFileServer(r *mux.Router) {
	stripped := http.StripPrefix(hp.URLPrefix, hp.srv)
	r.PathPrefix(hp.URLPrefix).Handler(stripped)
}

var staticHP = newHostedPath("static/", "/static/")

/// HTML templates.
// Need to be functions that return template.HTML rather than separate types
// with a custom String() function because html/template assumes all `string`s
// to be plaintext in need of HTML escaping.
/// --------------

func htmlFormattedTime(t time.Time, format string) template.HTML {
	utctime := t.UTC()
	str := utctime.Format(format)
	// Adding the `datetime` attribute in case we ever want to have some
	// JavaScript for conversion into different time zones…
	dt := utctime.Format(time.RFC3339)
	return template.HTML(fmt.Sprintf(`<time datetime="%s">%s</time>`, dt, str))
}

// HTMLTime outputs t in a nice HTML format.
func HTMLTime(t time.Time) template.HTML {
	return htmlFormattedTime(t, "2006-01-02 15:04&nbsp;UTC")
}

// HTMLDate outputs the date part of t in ISO 8601 format.
func HTMLDate(t time.Time) template.HTML {
	return htmlFormattedTime(t, "2006-01-02")
}

// HTMLEmoji returns HTML markup for the given custom emoji.
func HTMLEmoji(emoji string) template.HTML {
	fn := emoji
	style := ""
	switch emoji {
	case "onricdennat":
		fn = "tannedcirno"
		style = `transform: scaleX(-1);`
	}

	if len(style) > 0 {
		style = `style="` + style + `" `
	}
	return template.HTML(fmt.Sprintf(
		`<img src="%semoji-%s.png" alt=":%s:" %s/>`,
		staticHP.URLPrefix, fn, emoji, style,
	))
}

// HTMLFloatMaxPrec prints the most compact representation of v, with at
// most n decimal places.
func HTMLFloatMaxPrec(val float64, n int) string {
	pow := math.Pow10(n)
	return fmt.Sprintf("%v", math.Round(val*pow)/pow)
}

// HTMLPercentage formats the given value as a percentage.
func HTMLPercentage(val float64) template.HTML {
	if math.IsNaN(val) {
		return "n/a"
	} else if val == 100.0 {
		return "100&nbsp;%"
	}
	return template.HTML(fmt.Sprintf("%.2f&nbsp;%%", val))
}

// HTMLCurrency formats the given amount of cents as a currency value.
func HTMLCurrency(cents float64) template.HTML {
	return template.HTML(
		fmt.Sprintf(
			"<script>formatCurrency(%.0f)</script>"+
				"<noscript>%.2f&nbsp;€</noscript>", cents, cents/100.0,
		),
	)
}

// HTMLPushPrice returns the current push price, rendered as HTML.
func HTMLPushPrice() template.HTML {
	return HTMLCurrency(pushprices.Current())
}

// HTMLDownload returns a file download link for basename, hosted at hp.
func HTMLDownload(hp hostedPath, basename string) template.HTML {
	localFN := hp.LocalPath + basename
	fi, err := os.Stat(localFN)
	FatalIf(err)
	return template.HTML(
		fmt.Sprintf(
			`<a class="download" href="%s" data-kb="%.1f">%s </a>`,
			(hp.URLPrefix + basename), (float64(fi.Size()) / 1024.0), basename,
		),
	)
}

// HTMLTag returns a rendered blog tag, styled depending on its presence in the
// given filters, and with additional links to manipulate the filters.
func HTMLTag(tag string, filters []string) template.HTML {
	linkFor := func(allTags []string, title string, text string) string {
		url := "/blog"
		if len(allTags) >= 1 {
			url += `/tag/` + strings.Join(allTags, "/")
		}
		return `<a href="` + url + `" title="` + title + `">` + text + `</a>`
	}

	indexInFilters := -1
	for i, filter := range filters {
		if filter == tag {
			indexInFilters = i
			break
		}
	}

	body := linkFor([]string{tag}, tagDescriptions[tag], tag)
	class := "tag"

	// Any append() call that involves filters, or any sub-slice of it, will
	// modify the underlying array!
	if indexInFilters != -1 {
		class += " active"
		filterMod := append([]string{}, filters[:indexInFilters]...)
		filterMod = append(filterMod, filters[indexInFilters+1:]...)
		body += linkFor(filterMod, "Remove from filters", "-")
	} else if len(filters) >= 1 {
		filterMod := append([]string{}, filters...)
		filterMod = append(filterMod, tag)
		body += linkFor(filterMod, "Add to filters", "+")
	}
	return template.HTML(`<span class="` + class + `">` + body + `</span>`)
}

var pages = template.New("").Funcs(map[string]interface{}{
	// Arithmetic, safe
	"inc": func(i int) int { return i + 1 },

	// Control flow, safe
	"loop": func(from int, to int) chan int {
		ret := make(chan int)
		go func() {
			for i := from; i < to; i++ {
				ret <- i
			}
			close(ret)
		}()
		return ret
	},

	// Markup, safe
	"HTML_Date":         HTMLDate,
	"HTML_Time":         HTMLTime,
	"HTML_Emoji":        HTMLEmoji,
	"HTML_FloatMaxPrec": HTMLFloatMaxPrec,
	"HTML_Percentage":   HTMLPercentage,
	"HTML_Currency":     HTMLCurrency,
	"HTML_PushPrice":    HTMLPushPrice,
	"HTML_Download":     HTMLDownload,
	"HTML_Tag":          HTMLTag,

	// ReC98, safe
	"ReC98_REProgressAtTree": REProgressAtTree,
	"ReC98_REBaselineRev":    REBaselineRev,

	// Blog, safe
	"Blog_PostLink": PostLink,

	// PayPal, safe
	"PayPal_ClientID": func() string { return paypal_auth.ClientID },
})

// ParseSubdirectory parses all files in `dir` that match glob into
// subtemplates of t, prefixing their name with `dir` (unlike Go's own
// template.ParseGlob function), and returns a slice of the file names parsed.
func ParseSubdirectory(t *template.Template, dir string, glob string) (templates []string) {
	matches, err := filepath.Glob(filepath.Join(dir, glob))
	FatalIf(err)
	for _, m := range matches {
		buf, err := ioutil.ReadFile(m)
		FatalIf(err)
		template.Must(t.New(m).Parse(string(buf)))
	}
	return matches
}

// pagesExecute wraps template execution on [pages], logging any errors
// using the facilities from package log.
func pagesExecute(wr io.Writer, name string, data interface{}) {
	if err := pages.ExecuteTemplate(wr, name, data); err != nil {
		log.Println(wr, err)
	}
}

func respondWithError(wr http.ResponseWriter, err error) {
	wr.WriteHeader(500)
	fmt.Fprintln(wr, err)
}

// MuxedRequest wraps a http.Request together with the request vars from
// gorilla/mux.
type MuxedRequest struct {
	*http.Request
	Vars map[string]string
}

// NewMuxedRequest wraps req into a MuxedRequest.
func NewMuxedRequest(req *http.Request) MuxedRequest {
	return MuxedRequest{req, mux.Vars(req)}
}

// pagesHandler returns a handler that executes the given template of [pages],
// with a map of the request variables as the value of dot.
func pagesHandler(template string) http.Handler {
	tmpl := pages.Lookup(template)
	if tmpl == nil {
		log.Fatalf("couldn't find template %s", template)
	}

	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		if err := tmpl.Execute(wr, NewMuxedRequest(req)); err != nil {
			respondWithError(wr, err)
		}
	})
}

/// --------------

func measureRequestTime(next http.Handler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		start := time.Now()
		next.ServeHTTP(wr, req)
		log.Printf("%s %s: %v\n", req.Method, req.URL.Path, time.Since(start))
	})
}

func main() {
	if len(os.Args) < 2 {
		log.Fatalf("Usage: %s <ReC98 repository path/URL>\n", os.Args[0])
	}

	repo := NewRepository(os.Args[1])
	master, err := repo.GetCommit("master")
	if err != nil {
		log.Fatal(err)
	}
	pages.Funcs(map[string]interface{}{
		// Can fail
		"git_getCommit": repo.GetCommit,
		"git_getLogAt":  repo.GetLogAt,

		// Safe
		"git_commits":        commits,
		"git_makeCommitInfo": makeCommitInfo,
		"git_CommitLink":     CommitLink,
		"git_MasterCommit":   func() *object.Commit { return master },
	})

	// Late database initialization
	// ----------------------------
	pushes := NewPushes(transactions, tsvPushes, &repo)

	pages.Funcs(map[string]interface{}{
		"DB_CustomerByID": func(id CustomerID) template.HTML {
			return customers.HTMLByID(id)
		},
		"DB_TransactionBacklog": TransactionBacklog,
		"DB_Pushes":             pushes.All,
		"DB_CapCurrent":         CapCurrent,
	})
	// ----------------------------

	// Calculate the baseline for reverse-engineering progress
	// -------------------------------------------------------
	baselineFunc, err := REProgressBaseline(&repo)
	if err != nil {
		log.Fatalln("Error retrieving the baseline for reverse-engineering progress:", err)
	}
	log.Printf("That worked!")

	sppFunc := RESpeedPerPushFrom(pushes.DiffsForEstimate())
	estimateFunc := REProgressEstimateAtTree(baselineFunc())

	pages.Funcs(map[string]interface{}{
		"ReC98_REProgressBaseline":       baselineFunc,
		"ReC98_RESpeedPerPush":           sppFunc,
		"ReC98_REProgressEstimateAtTree": estimateFunc,
	})
	// -------------------------------------------------------

	// Blog
	// ----
	blog := NewBlog(pages, pushes, blogTags).AutogenerateTags(&repo)
	pages.Funcs(map[string]interface{}{
		"Blog_Posts":            blog.Posts,
		"Blog_FindEntryForPush": blog.FindEntryForPush,
		"Blog_GetPost":          blog.GetPost,
		"Blog_ParseTags": func(t string) []string {
			return strings.FieldsFunc(t, func(c rune) bool { return c == '/' })
		},
	})
	// ----

	pages = template.Must(pages.ParseGlob("*.html"))

	// Badge data
	// ----------
	masterTree, err := master.Tree()
	FatalIf(err)

	badger := Badger{
		Done:     REProgressAtTree(masterTree).Pct(baselineFunc()),
		Fallback: pagesHandler("badges.html"),
	}
	// ----------

	log.Printf("Got everything, starting the server.")

	r := mux.NewRouter()

	r.Use(measureRequestTime)
	staticHP.RegisterFileServer(r)
	blogHP.RegisterFileServer(r)
	r.Handle("/favicon.ico", staticHP.Server())
	r.Handle("/", pagesHandler("index.html"))
	r.Handle("/faq", pagesHandler("faq.html"))
	r.Handle("/fundlog", pagesHandler("fundlog.html"))
	r.Handle(blogURLPrefix, pagesHandler("blog.html"))
	r.Handle(blogURLPrefix+"/{date}", pagesHandler("blog_single.html"))
	r.Handle(
		blogURLPrefix+"/tag/{tags:(?:.+/?)+}", pagesHandler("blog_tagged.html"),
	)
	r.Handle("/progress", pagesHandler("progress.html"))
	r.Handle("/progress/{rev}", pagesHandler("progress_for.html"))
	if paypal_auth.Initialized() {
		r.Handle("/api/transaction-incoming", transactionIncomingHandler)
		r.Handle("/order", pagesHandler("order.html"))
		r.Handle("/thankyou", pagesHandler("thankyou.html"))
	}
	r.Handle("/badges", badger.Fallback)
	r.Handle("/badge/{type}", badger)
	r.Handle("/badge/{type}/{game}", badger)
	r.Handle("/legal", pagesHandler("legal.html"))
	log.Fatal(http.ListenAndServe(":8098", r))
}
