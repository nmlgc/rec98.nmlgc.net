package main

import (
	"encoding/csv"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gocarina/gocsv"
)

const dbPath = "db/"

/// Custom types
/// ------------

// DiffInfo contains all pieces of information parsed from a GitHub diff URL.
type DiffInfo struct {
	URL     string
	Project string
	Rev     string
}

// NewDiffInfo parses a GitHub diff URL into a DiffInfo structure.
func NewDiffInfo(url string) DiffInfo {
	s := strings.Split(url, "/")
	project := ""
	if s[1] == "rec98.nmlgc.net" {
		project = "Website"
	}
	return DiffInfo{
		URL:     url,
		Project: project,
		Rev:     s[len(s)-1],
	}
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
	Diff              string
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
