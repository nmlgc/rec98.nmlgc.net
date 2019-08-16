package main

import (
	"log"
	"strings"
	"time"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	"gopkg.in/src-d/go-git.v4/storage/memory"
)

var repo *git.Repository
var master *plumbing.Hash

type commitInfo struct {
	Date time.Time
	Desc string
	Hash plumbing.Hash
}

func makeCommitInfo(c *object.Commit) commitInfo {
	desc := c.Message
	if i := strings.IndexByte(desc, '\n'); i > 0 {
		desc = desc[:i]
	}
	return commitInfo{
		Date: c.Author.When,
		Desc: desc,
		Hash: c.Hash,
	}
}

func optimalClone(url string) error {
	var err error
	if strings.HasPrefix(url, "https://") {
		// TODO: This takes 3 GB of memory?!
		log.Printf("Cloning %s to memory...\n", url)
		repo, err = git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: url,
		})
	} else {
		log.Printf("Opening %s...\n", url)
		repo, err = git.PlainOpen(url)
	}
	return err
}
