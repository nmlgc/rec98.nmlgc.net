package main

import (
	"fmt"
	"html/template"
	"net/http"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

var htmlCommitRow = template.Must(template.New("commit-row").Parse(`<tr><td>{{.Date}}</td><th>{{.Desc}}</th><td><a href="https://github.com/nmlgc/ReC98/commit/{{.Hash}}"><code>{{.Hash}}</code></a></td></tr>`))

func indexHandler(wr http.ResponseWriter, req *http.Request) {
	masterIter, err := repo.Log(&git.LogOptions{From: *master})
	if err != nil {
		fmt.Fprintln(wr, err)
	}

	if err := pages.ExecuteTemplate(wr, "index.html", nil); err != nil {
		fmt.Fprintln(wr, err)
	}

	fmt.Fprintln(wr, "<table><tbody>")

	err = masterIter.ForEach(func(c *object.Commit) error {
		if err := htmlCommitRow.Execute(wr, makeCommitInfo(c)); err != nil {
			fmt.Fprintln(wr, err)
		}
		return nil
	})
	fmt.Fprintln(wr, "</tbody></table>")
	if err != nil {
		fmt.Fprintln(wr, err)
	}
}
