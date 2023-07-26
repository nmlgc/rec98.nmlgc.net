package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
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

var staticHP = NewHostedPath("static/", "/static/")

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
	ext := ".png"
	switch emoji {
	case "onricdennat":
		fn = "tannedcirno"
		style = `transform: scaleX(-1);`

	// SVG icons
	case "opencollective":
		fallthrough
	case "paypal":
		fallthrough
	case "stripe":
		ext = ".svg"
	}

	if len(style) > 0 {
		style = `style="` + style + `" `
	}
	url := staticHP.VersionURLFor("emoji-" + fn + ext)
	return template.HTML(fmt.Sprintf(
		// Calculated from the default `font-size` times `--icon-width` or
		// `--icon-height`.
		`<img src="%s" alt=":%s:" width="24" height="24" align="top" %s/>`,
		url, emoji, style,
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
func HTMLDownload(hp *HostedPath, basename string) template.HTML {
	localFN := hp.LocalPath + basename
	fi, err := os.Stat(localFN)
	FatalIf(err)
	return template.HTML(
		fmt.Sprintf(
			`<a class="download" href="%s" data-kb="%.1f">%s </a>`,
			hp.VersionURLFor(basename), (float64(fi.Size()) / 1024.0), basename,
		),
	)
}

func HTMLPC98Y(class string, title string, v int) template.HTML {
	return template.HTML(fmt.Sprintf(
		`<span class="y %s" title="%s">%v</span>`, class, title, v,
	))
}

// HTMLScreenY formats v as a Y coordinate in the PC-98 640×400 screen space.
func HTMLScreenY(v int) template.HTML {
	return HTMLPC98Y("screen", "Y coordinate in on-screen 640×400 space", v)
}

// HTML200Y formats v as a Y coordinate in the PC-98 VRAM space, in
// line-doubled 640×200 mode.
func HTML200Y(v int) template.HTML {
	return HTMLPC98Y("vram200", "Y coordinate in 640×200 VRAM space", v)
}

type eNoDescription struct {
	Tag string
}

func (e eNoDescription) Error() string {
	return fmt.Sprintf("no description for tag \"%s\"", e.Tag)
}

// HTMLTag returns a rendered blog tag, styled depending on its presence in the
// given filters, and with additional links to manipulate the filters.
func HTMLTag(tag string, filters []string) (template.HTML, error) {
	linkFor := func(allTags []string, title string, text string) string {
		url := "/blog/tag"
		if len(allTags) >= 1 {
			url += `/` + strings.Join(allTags, "/")
		}
		escTitle := template.HTMLEscapeString(title)
		return `<a href="` + url + `" title="` + escTitle + `">` + text + `</a>`
	}

	indexInFilters := -1
	for i, filter := range filters {
		if filter == tag {
			indexInFilters = i
			break
		}
	}

	desc, ok := tagDescriptions.Map[tag]
	if !ok {
		// Nope, no log.Fatalf() here, don't want a mistyped or outdated tag
		// filter URL to crash the entire server process.
		return "", eNoDescription{tag}
	}
	body := linkFor([]string{tag}, desc, tag)
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
	return template.HTML(`<span class="` + class + `">` + body + `</span>`), nil
}

var pages = template.New("").Funcs(map[string]interface{}{
	// Arithmetic, safe
	"pct": func(f float64) float64 { return (f * 100.0) },
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

	// String munging, safe
	"hasprefix": strings.HasPrefix,

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
	"HTML_Screen_Y":     HTMLScreenY,
	"HTML_200_Y":        HTML200Y,

	// ReC98, safe
	"ReC98_REProgressAtTree": REProgressAtTree,
	"ReC98_REBaselineRev":    REBaselineRev,
})

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

// PageDot bundles all data handed to a page template via dot.
type PageDot struct {
	*http.Request
	Vars          map[string]string // from gorilla/mux
	TemplateName  string
	StaticFileURL func(string) string
}

// NewPageDot builds a new PageDot structure.
func NewPageDot(req *http.Request, templateName string) PageDot {
	return PageDot{req, mux.Vars(req), templateName, func(fn string) string {
		return staticHP.VersionURLFor(fn)
	}}
}

// pagesHandler returns a handler that executes the given template of [pages],
// with a map of the request variables as the value of dot.
func pagesHandler(template string) http.Handler {
	header := pages.Lookup("header.html")
	footer := pages.Lookup("footer.html")
	tmpl := pages.Lookup(template)
	if tmpl == nil {
		log.Fatalf("couldn't find template %s", template)
	}

	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		errorIf := func(err error) {
			if err != nil {
				respondWithError(wr, err)
			}
		}
		dot := NewPageDot(req, strings.TrimSuffix(template, path.Ext(template)))
		errorIf(header.Execute(wr, dot))
		errorIf(tmpl.Execute(wr, dot))
		errorIf(footer.Execute(wr, dot))
	})
}

type ProviderHandler = func(http.ResponseWriter, *http.Request, *Incoming)

// incomingHandler wraps the given provider-specific payment processing handler
// with common request validation and transaction parsing code.
func incomingHandler(handler ProviderHandler) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		var in Incoming
		err := json.NewDecoder(req.Body).Decode(&in)
		if err != nil {
			respondWithError(wr, err)
			return
		}
		handler(wr, req, &in)
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
	domainString := os.Getenv("DOMAIN")
	if len(os.Args) < 2 || domainString == "" {
		log.Fatalf(
			"Usage: DOMAIN=<public URL> %s <ReC98 repository path/URL>\n",
			os.Args[0],
		)
	}
	domain, err := url.ParseRequestURI(domainString)
	FatalIf(err)

	// Concurrent initialization
	// -------------------------
	paypalClient := Concurrent(NewPaypalClient)
	videoRoot := Concurrent(func() *VideoRoot {
		return NewVideoRoot(SymmetricPath{
			LocalPath: filepath.Join(blogHP.LocalPath, "video"),
			URLPrefix: "video/", // blogHP contains the remaining prefix
		})
	})
	// -------------------------

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
		"DB_DiscountOffers": func() []DiscountOfferView {
			return DiscountOffers(pushprices.Current())
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
	blog := NewBlog(pages, pushes, blogTags, <-videoRoot, func(blog *Blog) map[string]interface{} {
		return map[string]interface{}{
			"Blog_Posts":            blog.Posts,
			"Blog_PostLink":         blog.PostLink,
			"Blog_FindEntryForPush": blog.FindEntryForPush,
			"Blog_GetPost":          blog.GetPost,
			"Blog_ParseTags": func(t string) []string {
				return strings.FieldsFunc(t, func(c rune) bool {
					return c == '/'
				})
			},
			"Blog_TagDescriptions": func() []*TagDescription {
				return tagDescriptions.Ordered
			},
		}
	}).AutogenerateTags(&repo)
	feedHandler := FeedHandler{
		Blog:     blog,
		SiteURL:  domain.String(),
		BlogPath: "/blog",
	}
	// ----

	// Payment providers
	// -----------------
	paypal := <-paypalClient
	if paypal != nil {
		pages.Funcs(map[string]interface{}{
			"PayPal_ClientID": func() string { return paypal.ClientID },
		})
	}
	stripe := NewStripeClient(domain, "/api/stripe", "/customer/stripe")
	if stripe != nil {
		pages.Funcs(map[string]interface{}{
			"Stripe_Session":   stripe.Session,
			"Stripe_SubVerify": stripe.Sub,
			"Stripe_RouteAPICancel": func() string {
				return stripe.RouteAPICancel
			},
		})
	}
	// -----------------

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
	r.Handle(
		blogHP.URLPrefix+"{stem}-vp8.webm", blog.OldVideoRedirectHandler(&VP8),
	)
	r.Handle(blogHP.URLPrefix+"{stem}.webm", blog.OldVideoRedirectHandler(&VP9))
	staticHP.RegisterFileServer(r)
	blogHP.RegisterFileServer(r)
	r.Handle("/favicon.ico", staticHP.Server())
	r.Handle("/", pagesHandler("index.html"))
	r.Handle("/faq", pagesHandler("faq.html"))
	r.Handle("/fundlog", pagesHandler("fundlog.html"))
	r.Handle(blogURLPrefix, pagesHandler("blog.html"))
	r.Handle(blogURLPrefix+"/feed.xml", http.HandlerFunc(feedHandler.HandleRSS))
	r.Handle(blogURLPrefix+"/feed.atom", http.HandlerFunc(feedHandler.HandleAtom))
	r.Handle(blogURLPrefix+"/feed.json", http.HandlerFunc(feedHandler.HandleJSON))
	r.Handle(blogURLPrefix+"/tag", pagesHandler("blog_taglist.html"))
	r.Handle(blogURLPrefix+"/{date}", pagesHandler("blog_single.html"))
	r.Handle(
		blogURLPrefix+"/tag/{tags:(?:.+/?)+}", pagesHandler("blog_tagged.html"),
	)
	r.Handle("/progress", pagesHandler("progress.html"))
	r.Handle("/progress/{rev}", pagesHandler("progress_for.html"))
	if len(providerAuth) > 0 {
		r.Handle("/order", pagesHandler("order.html"))
		r.Handle("/thankyou", pagesHandler("thankyou.html"))

		if paypal != nil {
			r.Methods("POST").Path("/api/paypal/incoming").
				Handler(incomingHandler(PaypalIncomingHandler(paypal)))
		}
		if stripe != nil {
			r.Methods("POST").Path(stripe.RouteAPIIncoming).
				Handler(incomingHandler(stripe.HandleIncoming))

			// Yup, Stripe does in fact send a GET request to an endpoint that
			// is highly likely to change a server's database…
			r.Handle(
				stripe.RouteAPISuccess, http.HandlerFunc(stripe.HandleSuccess),
			)

			r.Methods("POST").Path(stripe.RouteAPICancel).
				Handler(http.HandlerFunc(stripe.HandleCancel))

			r.Handle(
				stripe.RoutePageThankYou+"/{stripeSession}/{salt}",
				pagesHandler("thankyou.html"),
			)
			r.Handle(
				stripe.RoutePageManage+"/{salt}",
				pagesHandler("customer_stripe.html"),
			)
		}
	} else {
		log.Println("`provider_auth` table is empty, disabling order pages.")
	}
	r.Handle("/donate", pagesHandler("donate.html"))
	r.Handle("/badges", badger.Fallback)
	r.Handle("/badge/{type}", badger)
	r.Handle("/badge/{type}/{game}", badger)
	r.Handle("/legal", pagesHandler("legal.html"))
	log.Fatal(http.ListenAndServe(":8098", r))
}
