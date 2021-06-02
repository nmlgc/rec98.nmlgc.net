package main

import (
	"fmt"
	"html/template"
	"log"
	"strings"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

// Repository wraps a go-git Repository object.
type Repository struct {
	R *git.Repository

	UniqueLen         int // Shortest possible but still unique commit hash length
	ShortHashToCommit map[string]*object.Commit
}

type eHashTooShort struct{}

func (e eHashTooShort) Error() string {
	return ""
}

func testShortLength(r *git.Repository, sl int) map[string]*object.Commit {
	ret := make(map[string]*object.Commit)
	commitIter, err := r.CommitObjects()
	FatalIf(err)
	err = commitIter.ForEach(func(c *object.Commit) error {
		curFull := c.Hash
		curShort := curFull.String()[:sl]
		if _, ok := ret[curShort]; ok {
			return eHashTooShort{}
		}
		ret[curShort] = c
		return nil
	})
	if _, ok := err.(eHashTooShort); ok {
		return nil
	}
	return ret
}

// NewRepository calls optimalClone(url), and wraps the result into our
// Repository type.
func NewRepository(url string) (ret Repository) {
	r, err := optimalClone(url)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Done.")

	testlen := 0
	for testlen < 40 && ret.ShortHashToCommit == nil {
		testlen++
		ret.ShortHashToCommit = testShortLength(r, testlen)
	}
	log.Printf("shortest unique hash length is %v characters", testlen)

	ret.UniqueLen = testlen
	ret.R = r
	return
}

// GetCommit returns a potential Commit object for the given rev.
func (r *Repository) GetCommit(rev string) (*object.Commit, error) {
	if len(rev) >= r.UniqueLen {
		if commit, ok := r.ShortHashToCommit[rev[:r.UniqueLen]]; ok {
			// Verify that the rest of the hash matches what we expect, and
			// fall through to ResolveRevision otherwise.
			if rev == commit.Hash.String()[:len(rev)] {
				return commit, nil
			}
		}
	}
	hash, err := r.R.ResolveRevision(plumbing.Revision(rev))
	if err == nil {
		return r.R.CommitObject(*hash)
	}
	return nil, err
}

// GetLogAt returns an in-order log iterator for the given rev.
func (r *Repository) GetLogAt(c *object.Commit) (object.CommitIter, error) {
	return r.R.Log(&git.LogOptions{From: c.Hash})
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

func optimalClone(url string) (*git.Repository, error) {
	if strings.HasPrefix(url, "https://") {
		// TODO: This takes 3 GB of memory?!
		log.Printf("Cloning %s to memory...\n", url)
		return git.Clone(memory.NewStorage(), nil, &git.CloneOptions{
			URL: url,
		})
	}
	log.Printf("Opening %s...\n", url)
	return git.PlainOpen(url)
}
