package main

import (
	"fmt"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
	"gopkg.in/src-d/go-git.v4/plumbing"
)

var htmlCommitH3 = template.Must(template.New("commit-h3").Parse(`<h3>{{.Date}} – {{.Desc}} (<a href="https://github.com/nmlgc/ReC98/commit/{{.Hash}}"><code>{{.Hash}}</code></a>)</h3>`))

func numbersHandler(wr http.ResponseWriter, req *http.Request) {
	hashIn := mux.Vars(req)["rev"]

	hash, err := repo.ResolveRevision(plumbing.Revision(hashIn))
	if err != nil {
		fmt.Fprintln(wr, err)
	}

	commit, err := repo.CommitObject(*hash)
	if err != nil {
		fmt.Fprintln(wr, err)
	}

	tree, err := commit.Tree()
	if err != nil {
		fmt.Fprintln(wr, err)
	}

	fmt.Fprintln(wr, "<h2>Commit overview</h2>")

	if err := htmlCommitH3.Execute(wr, makeCommitInfo(commit)); err != nil {
		fmt.Fprintln(wr, err)
	}

	fmt.Fprintln(wr, "<table><thead><tr><th></th><th>ZUN.COM</th><th>OP</th><th>Main</th><th>Cutscenes</th></tr></thead><tbody>")
	for game, sources := range gameSources {
		fmt.Fprint(wr, "<tr>")
		fmt.Fprintf(wr, "<th>TH%02d</th>", game+1)
		fmt.Fprintf(wr, "<td>%v</td>", numbersOfSources(tree, sources.Init))
		fmt.Fprintf(wr, "<td>%v</td>", numbersOfSources(tree, sources.OP))
		fmt.Fprintf(wr, "<td>%v</td>", numbersOfSources(tree, sources.Main))
		fmt.Fprintf(wr, "<td>%v</td>", numbersOfSources(tree, sources.Cutscenes))
		fmt.Fprint(wr, "</tr>")
	}
	fmt.Fprintln(wr, "</tbody></table>")
}
