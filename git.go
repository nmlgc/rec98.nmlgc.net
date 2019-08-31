package main

import (
	"fmt"
	"html/template"
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

func getCommit(rev string) (*object.Commit, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(rev))
	if err != nil {
		return nil, err
	}
	return repo.CommitObject(*hash)
}

func getLog() (object.CommitIter, error) {
	return repo.Log(&git.LogOptions{From: *master})
}

type commitInfo struct {
	Time time.Time
	Desc string
	Hash plumbing.Hash
}

func makeCommitInfo(c *object.Commit) commitInfo {
	desc := c.Message
	if i := strings.IndexByte(desc, '\n'); i > 0 {
		desc = desc[:i]
	}
	return commitInfo{
		Time: c.Author.When,
		Desc: desc,
		Hash: c.Hash,
	}
}

// CommitLink returns a nicely formatted link to rev in the ReC98 repository.
func CommitLink(rev string) template.HTML {
	revEsc := template.HTMLEscapeString(rev)
	return template.HTML(fmt.Sprintf(
		`<a href="https://github.com/nmlgc/ReC98/commit/%s"><code>%s</code></a>`,
		revEsc, revEsc,
	))
}

func commits(iter object.CommitIter) chan commitInfo {
	ret := make(chan commitInfo)
	go func() {
		err := iter.ForEach(func(c *object.Commit) error {
			ret <- makeCommitInfo(c)
			return nil
		})
		close(ret)
		if err != nil {
			log.Panicln(err)
		}
	}()
	return ret
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
