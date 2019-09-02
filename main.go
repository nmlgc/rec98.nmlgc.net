package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"time"

	"github.com/gorilla/mux"
)

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

/// HTML templates
/// --------------

// HTMLTime outputs a time.Time in a nice HTML format. Needs to be a function
// rather than a separate type with a custom String() function because that
// one always has to return a `string`, not a `template.HTML`.
func HTMLTime(t time.Time) template.HTML {
	utctime := t.UTC()
	str := utctime.Format("2006-01-02 15:04&nbsp;UTC")
	// Adding the `datetime` attribute in case we ever want to have some
	// JavaScript for conversion into different time zones…
	dt := utctime.Format(time.RFC3339)
	return template.HTML(fmt.Sprintf(`<time datetime="%s">%s</time>`, dt, str))
}

var pages = template.Must(template.New("").Funcs(map[string]interface{}{
	// Git, initialization
	"git_getCommit": getCommit,
	"git_getLog":    getLog,

	// Arithmetic, safe
	"inc": func(i int) int { return i + 1 },

	// Markup, safe
	"HTML_Time": HTMLTime,

	// Git, safe
	"git_commits":        commits,
	"git_makeCommitInfo": makeCommitInfo,
	"git_CommitLink":     CommitLink,

	// ReC98, safe
	"ReC98_REProgressAtTree": REProgressAtTree,
	"ReC98_REBaselineRev":    REBaselineRev,
	// Added after the repository was successfully opened
	"ReC98_REProgressBaseline": func() int { return 0 },

	// Database view, safe
	"DB_CustomerByID":      CustomerByID,
	"DB_PushesOutstanding": PushesOutstanding,
	"DB_PushesDelivered":   PushesDelivered,
}).ParseGlob("*.html"))

// pagesExecute wraps template execution on [pages], logging any errors
// using the facilities from package log.
func pagesExecute(wr io.Writer, name string, data interface{}) {
	if err := pages.ExecuteTemplate(wr, name, data); err != nil {
		log.Println(wr, err)
	}
}

// pagesHandler returns a handler that executes the given template of [pages],
// with a map of the request variables as the value of dot.
func pagesHandler(template string) http.Handler {
	tmpl := pages.Lookup(template)
	if tmpl == nil {
		log.Fatalf("couldn't find template %s", template)
	}

	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
		if err := tmpl.Execute(wr, mux.Vars(req)); err != nil {
			wr.WriteHeader(500)
			fmt.Fprintln(wr, err)
			return
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

	err := optimalClone(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Done.")

	master, err = repo.ResolveRevision("master")
	if err != nil {
		log.Fatal(err)
	}

	// Calculate the baseline for reverse-engineering progress
	// -------------------------------------------------------
	baselineFunc, err := REProgressBaseline(repo)
	if err != nil {
		log.Fatalln("Error retrieving the baseline for reverse-engineering progress:", err)
	}
	log.Printf("That worked!")

	pages.Funcs(map[string]interface{}{
		"ReC98_REProgressBaseline": baselineFunc,
	})
	// -------------------------------------------------------

	log.Printf("Got everything, starting the server.")

	r := mux.NewRouter()

	r.Use(measureRequestTime)
	staticHP.RegisterFileServer(r)
	r.Handle("/favicon.ico", staticHP.Server())
	r.Handle("/", pagesHandler("index.html"))
	r.Handle("/fundlog", pagesHandler("fundlog.html"))
	r.Handle("/progress/{rev}", pagesHandler("progress.html"))
	log.Fatal(http.ListenAndServe(":8098", r))
}
