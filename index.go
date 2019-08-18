package main

import (
	"fmt"
	"log"
	"net/http"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

func commitLog() (chan commitInfo, error) {
	masterIter, err := repo.Log(&git.LogOptions{From: *master})
	if err != nil {
		return nil, err
	}

	ret := make(chan commitInfo)
	go func() {
		err := masterIter.ForEach(func(c *object.Commit) error {
			ret <- makeCommitInfo(c)
			return nil
		})
		close(ret)
		if err != nil {
			log.Panicln(err)
		}
	}()
	return ret, nil
}

func indexHandler(wr http.ResponseWriter, req *http.Request) {
	if err := pages.ExecuteTemplate(wr, "index.html", nil); err != nil {
		fmt.Fprintln(wr, err)
	}
}
