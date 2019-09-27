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

type eInvalidID struct {
	input string
}

func (e eInvalidID) Error() string {
	return fmt.Sprintf("invalid ID: \"%s\"", e.input)
}

func parseID(input string, format string) (ret int, err error) {
	n, err := fmt.Sscanf(input, format, &ret)
	if err != nil {
		return 0, err
	} else if n != 1 {
		return 0, eInvalidID{input}
	}
	return ret, nil
}

/// ------------

/// Schemas
/// -------

// CustomerID represents a consecutively numbered, 1-based customer ID.
type CustomerID int

// PushID represents a consecutively numbered, 1-based customer ID.
type PushID int

const pushIDFormat = "P%04d"

func (i PushID) String() string {
	return fmt.Sprintf(pushIDFormat, i)
}

// UnmarshalCSV decodes a PushID from its string representation.
func (i *PushID) UnmarshalCSV(s string) error {
	ret, err := parseID(s, pushIDFormat)
	*i = PushID(ret)
	return err
}

// Customer represents everyone who bought something.
type Customer struct {
	Name string
	URL  string
}

// Push represents a single unit of work.
type Push struct {
	ID                PushID
	Purchased         time.Time
	Customer          CustomerID
	Goal              string
	Delivered         NullableTime
	Summary           *string
	Diff              *DiffInfo
	IncludeInEstimate bool
}

// PushPrice represents the price of one push at a given point in time.
type PushPrice struct {
	Time  time.Time
	Cents int
}

type tCustomers []*Customer
type tPushes []*Push
type tPushPrices []*PushPrice

func (c tCustomers) ByID(id CustomerID) Customer {
	return *c[id-1]
}

func (p tPushPrices) At(t time.Time) (price int) {
	for _, pushprice := range p {
		if pushprice.Time.Before(t) {
			price = pushprice.Cents
		}
	}
	return
}

var customers = tCustomers{}
var pushes = tPushes{}
var pushprices = tPushPrices{}

/// -------

// TSV input structures
// --------------------
type pushTSV struct {
	ID                PushID
	Purchased         time.Time
	Customer          CustomerID
	Goal              string
	Delivered         NullableTime
	Diff              *DiffInfo
	IncludeInEstimate bool
}

func (p *pushTSV) toActualPush() *Push {
	var summary *string
	if p.Delivered.Time != nil {
		summary = blog.HasEntryFor(*p.Delivered.Time)
	}
	return &Push{
		ID:                p.ID,
		Purchased:         p.Purchased,
		Customer:          p.Customer,
		Goal:              p.Goal,
		Delivered:         p.Delivered,
		Diff:              p.Diff,
		IncludeInEstimate: p.IncludeInEstimate,

		Summary: summary,
	}
}

// --------------------

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
	var tsvPushes []*pushTSV

	err = loadTSV(&customers, "customers")
	if err != nil {
		log.Fatalln(err)
	}
	err = loadTSV(&tsvPushes, "pushes")
	if err != nil {
		log.Fatalln(err)
	}
	err = loadTSV(&pushprices, "pushprices")
	if err != nil {
		log.Fatalln(err)
	}
	for _, p := range tsvPushes {
		pushes = append(pushes, p.toActualPush())
	}
}
