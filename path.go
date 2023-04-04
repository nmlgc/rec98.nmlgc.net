package main

import (
	"encoding/hex"
	"log"
	"net/http"
	"path/filepath"
	"sync"

	"github.com/gorilla/mux"
	"github.com/rjeczalik/notify"
)

// SymmetricPath describes a local filesystem path that can be accessed via
// HTTP through a URL prefix.
type SymmetricPath struct {
	LocalPath string // must end with a slash
	URLPrefix string // must start *and* end with a slash
}

// HostedPath adds an HTTP file server with version-based URL suffixes to
// SymmetricPath.
type HostedPath struct {
	SymmetricPath
	srv         http.Handler
	fileToHash  sync.Map
	depToSource sync.Map
}

// NewHostedPath sets up a new HostedPath instance.
func NewHostedPath(LocalPath string, URLPrefix string) *HostedPath {
	absoluteLocalPath, err := filepath.Abs(LocalPath)
	FatalIf(err)
	dir := http.FileServer(http.Dir(absoluteLocalPath))
	ret := &HostedPath{
		SymmetricPath: SymmetricPath{
			LocalPath: (absoluteLocalPath + "/"),
			URLPrefix: URLPrefix,
		},
		srv: http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
			if len(req.URL.RawQuery) > 0 {
				wr.Header().Set("Cache-Control", "max-age=31536000, immutable")
			}
			dir.ServeHTTP(wr, req)
		}),
	}
	go ret.watch()
	return ret
}

// watch watches the LocalPath for file changes in order to purge hashes as
// necessary.
func (hp *HostedPath) watch() {
	// Make the channel buffered to ensure no event is dropped. Notify will
	// drop an event if the receiver is not able to keep up the sending pace.
	c := make(chan notify.EventInfo, 1)

	err := notify.Watch(hp.LocalPath+"...", c, notify.Write)
	FatalIf(err)

	for e := range c {
		fn, err := filepath.Rel(hp.LocalPath, e.Path())
		FatalIf(err)

		// At least on Windows, I always get multiple events each time I save a
		// file. At the first event, the file is still locked for concurrent
		// reads, so we can't immediately rehash it. So, let's just delete the
		// old hash, and let the new one be generated on demand…
		if _, deleted := hp.fileToHash.LoadAndDelete(fn); deleted {
			log.Printf("%s: \"%s\" changed", hp.LocalPath, fn)
			if dep, ok := hp.depToSource.Load(fn); ok {
				hp.fileToHash.Delete(dep)
			}
		}
	}
}

// Server returns hp's file serving handler, e.g. to be re-used elsewhere.
func (hp *HostedPath) Server() http.Handler {
	return hp.srv
}

// RegisterFileServer registers a HTTP route on the given router at hp's
// URLPrefix, serving any local files in hp's LocalPath.
func (hp *HostedPath) RegisterFileServer(r *mux.Router) {
	stripped := http.StripPrefix(hp.URLPrefix, hp.srv)
	r.PathPrefix(hp.URLPrefix).Handler(stripped)
}

// buildFile runs any necessary build step to generate fn. Returns an array of
// all existing files that should invalidate fn if they are changed.
func (hp *HostedPath) buildFile(fn string) (deps []string) {
	return append(deps, fn)
}

// VersionQueryFor returns the current hash of fn as a query string suffix.
func (hp *HostedPath) VersionQueryFor(fn string) string {
	hash, ok := hp.fileToHash.Load(fn)
	if !ok {
		deps := hp.buildFile(fn)
		for _, dep := range deps {
			hp.depToSource.Store(dep, fn)
			fullHash := CryptHashOfFile(hp.LocalPath + dep)
			hash = hex.EncodeToString(fullHash[:4])
			hp.fileToHash.Store(dep, hash)
		}
	}
	return "?" + hash.(string)
}

// VersionURLFor returns the full URL of fn, with a version-based query string
// suffix.
func (hp *HostedPath) VersionURLFor(fn string) string {
	return (hp.URLPrefix + fn + hp.VersionQueryFor(fn))
}
