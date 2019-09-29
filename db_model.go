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
const devLocationName = "Europe/Berlin"

var devLocation *time.Location

// FreeTimeDecisionPoint defines the cut-off time for deciding whether a day
// will be spent working on ReC98 or not.
const FreeTimeDecisionPoint = time.Duration(time.Hour * 16)

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

// LocalDateStamp represents a date-only timestamp in the devLocation.
type LocalDateStamp struct {
	time.Time
}

// UnmarshalCSV decodes an ISO 8601 date to a LocalDateStamp.
func (d *LocalDateStamp) UnmarshalCSV(s string) (err error) {
	d.Time, err = time.ParseInLocation("2006-01-02", s, devLocation)
	d.Time = d.Time.Add(FreeTimeDecisionPoint)
	return err
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

// TransactionID represents a consecutively numbered transaction ID.
type TransactionID int

const transactionIDFormat = "T%04d"

func (i TransactionID) String() string {
	return fmt.Sprintf(transactionIDFormat, i)
}

// UnmarshalCSV decodes a TransactionID from its string representation.
func (i *TransactionID) UnmarshalCSV(s string) error {
	ret, err := parseID(s, transactionIDFormat)
	*i = TransactionID(ret)
	return err
}

// Customer represents everyone who bought something.
type Customer struct {
	Name string
	URL  string
}

// Transaction represents a single money transfer that may or may not be large
// enough to result in one or more pushes.
type Transaction struct {
	ID       TransactionID
	Time     time.Time
	Customer CustomerID
	Cents    int
	Goal     string

	// Calculated after the push table has loaded
	Outstanding int
}

// Consumes up to pushpriceRem outstanding cents from p, and returns the new
// remaining push price.
func (t *Transaction) consume(p *pushTSV, pushpriceRem int) int {
	if pushpriceRem <= 0 {
		log.Fatalf(
			"%s consumed more transactions than it should have (%s)",
			p.ID, p.Transactions,
		)
	} else if t.Outstanding <= 0 {
		log.Fatalf("more pushes associated with %s than it paid for", t.ID)
	}
	if t.Outstanding >= pushpriceRem {
		t.Outstanding -= pushpriceRem
		return 0
	}
	pushpriceRem -= t.Outstanding
	t.Outstanding = 0
	return pushpriceRem
}

// Push represents a single unit of work.
type Push struct {
	ID                PushID
	Transactions      []*Transaction
	Goal              string
	Delivered         time.Time
	Summary           *string
	Diff              *DiffInfo
	IncludeInEstimate bool
}

// FundedBy returns all customers that were involved in funding this push
func (p Push) FundedBy() (ret []CustomerID) {
	for _, t := range p.Transactions {
		ret = append(ret, t.Customer)
	}
	RemoveDuplicates(&ret)
	return
}

// FundedAt returns the timestamp of the last transaction that was part of
// this push.
func (p Push) FundedAt() (ret time.Time) {
	for _, t := range p.Transactions {
		if t.Time.After(ret) {
			ret = t.Time
		}
	}
	return
}

// PushPrice represents the price of one push at a given point in time.
type PushPrice struct {
	Time  time.Time
	Cents int
}

// FreeTime represents a single day that can be spent on getting a push done.
type FreeTime struct {
	Date LocalDateStamp
}

type tCustomers []*Customer
type tTransactions []*Transaction
type tPushes []*Push
type tPushPrices []*PushPrice
type tFreeTime []*FreeTime

func (c tCustomers) ByID(id CustomerID) Customer {
	return *c[id-1]
}

func (t tTransactions) ByID(id TransactionID) *Transaction {
	return t[id-1]
}

func (p tPushPrices) At(t time.Time) (price int) {
	for _, pushprice := range p {
		if pushprice.Time.Before(t) {
			price = pushprice.Cents
		}
	}
	return
}

func (p tPushPrices) Current() (price float64) {
	return float64(p.At(time.Now()))
}

var customers = tCustomers{}
var transactions = tTransactions{}
var pushes = tPushes{}
var pushprices = tPushPrices{}
var freetime = tFreeTime{}

/// -------

// TSV input structures
// --------------------
type pushTSV struct {
	ID                PushID
	Transactions      []TransactionID
	Goal              string
	Delivered         time.Time
	Diff              *DiffInfo
	IncludeInEstimate bool
}

func (p *pushTSV) toActualPush() *Push {
	return &Push{
		ID:                p.ID,
		Goal:              p.Goal,
		Delivered:         p.Delivered,
		Diff:              p.Diff,
		IncludeInEstimate: p.IncludeInEstimate,

		Transactions: func() (ret []*Transaction) {
			if len(p.Transactions) == 0 {
				log.Fatalf("%s has no transactions associated with it", p.ID)
			}
			earliest := time.Now()
			for _, tid := range p.Transactions {
				t := transactions.ByID(tid)
				if t.Time.Before(earliest) {
					earliest = t.Time
				}
				ret = append(ret, t)
			}
			pushpriceRem := pushprices.At(earliest)
			for _, t := range ret {
				pushpriceRem = t.consume(p, pushpriceRem)
			}
			if pushpriceRem != 0 {
				log.Fatalf("%s is not fully paid for", p.ID)
			}
			return
		}(),
		Summary: blog.HasEntryFor(p.Delivered),
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

	devLocation, err = time.LoadLocation(devLocationName)
	if err != nil {
		log.Fatalln(err)
	}

	err = loadTSV(&customers, "customers")
	if err != nil {
		log.Fatalln(err)
	}
	err = loadTSV(&transactions, "transactions")
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
	err = loadTSV(&freetime, "freetime")
	if err != nil {
		log.Fatalln(err)
	}
	for i := range transactions {
		transactions[i].Outstanding = transactions[i].Cents
	}
	for _, p := range tsvPushes {
		pushes = append(pushes, p.toActualPush())
	}
}
