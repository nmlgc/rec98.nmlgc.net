package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const dbPath = "db/"

/// Custom types
/// ------------

// DiffInfo contains all pieces of information parsed from a GitHub diff URL.
type DiffInfo struct {
	URL     string
	Project string
	Rev     string
	IsRange bool
}

type eInvalidDiffURL struct {
	url string
	err string
}

func (e eInvalidDiffURL) Error() string {
	return fmt.Sprintf("invalid diff URL format: \"%s\": %s", e.url, e.err)
}

// UnmarshalCSV parses a GitHub diff URL into a DiffInfo structure.
func (d *DiffInfo) UnmarshalCSV(url string) error {
	if len(url) == 0 {
		return nil
	}
	s := strings.Split(url, "/")
	if len(s) != 4 {
		return eInvalidDiffURL{url, "expected 3 slashes"}
	}
	project := ""
	if s[1] == "rec98.nmlgc.net" {
		project = "Website"
	}
	var isRange bool
	switch s[2] {
	case "compare":
		isRange = true
	case "commit":
		isRange = false
	default:
		return eInvalidDiffURL{url,
			"mode must be either \"compare\" or \"commit\"",
		}
	}
	*d = DiffInfo{
		URL:     url,
		Project: project,
		Rev:     s[3],
		IsRange: isRange,
	}
	return nil
}

// Range retrieves the commits at the top and bottom of the range described by
// d.
func (d *DiffInfo) Range() (top, bottom *object.Commit) {
	must := func(ret *object.Commit, err error) *object.Commit {
		if err != nil {
			log.Fatalln(err)
		}
		return ret
	}

	if d.IsRange {
		revs := strings.Split(d.Rev, "...")
		if len(revs) == 1 && strings.Contains(d.Rev, "..") {
			log.Fatalln("two-dot ranges not supported:", d.Rev)
		}
		bottom = must(getCommit(revs[0]))
		top = must(getCommit(revs[1]))
		return
	}
	// Single commit...
	top = must(getCommit((d.Rev)))
	if len(top.ParentHashes) > 1 {
		log.Fatalf(
			"%s has more than one parent; use the \"compare\" mode instead!",
			d.Rev,
		)
	}
	bottom = must(top.Parent(0))
	return
}

// NullableTime represents time values that can be empty.
type NullableTime struct {
	*time.Time
}

// UnmarshalCSV wraps time.Parse to accept empty strings.
func (t *NullableTime) UnmarshalCSV(s string) error {
	t.Time = nil
	if len(s) == 0 {
		return nil
	}
	ret, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	t.Time = &ret
	return err
}

func (t NullableTime) String() string {
	if t.Time == nil {
		return ""
	}
	return t.Time.String()
}

/// ------------

/// Schemas
/// -------

// CustomerID represents a consecutively numbered customer ID.
type CustomerID int

// Customer represents everyone who bought something.
type Customer struct {
	Name string
	URL  string
}

// Push represents a single unit of work.
type Push struct {
	ID                string
	Purchased         time.Time
	Customer          CustomerID
	Goal              string
	Delivered         NullableTime
	Summary           *string
	Diff              *DiffInfo
	IncludeInEstimate bool
}

type tCustomers []*Customer
type tPushes []*Push

func (c tCustomers) ByID(id CustomerID) Customer {
	return *c[id-1]
}

var customers = tCustomers{}
var pushes = tPushes{}

/// -------

func loadTSV(slice interface{}, table string) error {
	f, err := os.Open(filepath.Join(dbPath, table+".tsv"))
	if err != nil {
		return err
	}
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	return gocsv.UnmarshalCSV(reader, slice)
}

func init() {
	var err error
	err = loadTSV(&customers, "customers")
	if err != nil {
		log.Fatalln(err)
	}
	err = loadTSV(&pushes, "pushes")
	if err != nil {
		log.Fatalln(err)
	}
	for _, p := range pushes {
		if p.Delivered.Time == nil {
			continue
		}
		if summary := blog.HasEntryFor(*p.Delivered.Time); summary != nil {
			p.Summary = summary
		}
	}
}
