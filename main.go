package main

import (
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

/// HTML templates
/// --------------

var pages = template.Must(template.New("").Funcs(map[string]interface{}{
	// Git, initialization
	"git_getCommit": getCommit,
	"git_getLog":    getLog,

	// Arithmetic, safe
	"inc": func(i int) int { return i + 1 },

	// Git, safe
	"git_commits":        commits,
	"git_makeCommitInfo": makeCommitInfo,

	// ReC98, safe
	"rec98_numbersAtTree": numbersAtTree,
}).ParseGlob("*.html"))

// executeTemplate wraps template execution on [pages], logging any errors
// using the facilities from package log.
func executeTemplate(wr io.Writer, name string, data interface{}) {
	if err := pages.ExecuteTemplate(wr, name, data); err != nil {
		log.Println(wr, err)
	}
}

func htmlWrap(template string) http.Handler {
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

	r := mux.NewRouter()

	staticSrv := http.FileServer(http.Dir("static/"))

	r.Use(measureRequestTime)
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", staticSrv))
	r.Handle("/favicon.ico", staticSrv)
	r.Handle("/", htmlWrap("index.html"))
	r.Handle("/numbers/{rev}", htmlWrap("numbers.html"))
	log.Fatal(http.ListenAndServe(":8098", r))
}
