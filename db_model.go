package main

import (
	"encoding/csv"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gocarina/gocsv"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const dbPath = "db/"
const devLocationName = "Europe/Berlin"

var devLocation *time.Location

// CapWindow defines the maximum acceptable backlog size, based on the free
// time from now to this point in the future.
const CapWindow = time.Duration(time.Hour * 24 * 7 * 4)

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
		FatalIf(err)
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
	Summary           *BlogEntry
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

// Incoming represents an unprocessed order coming in from the client side.
type Incoming struct {
	PayPalID string
	CustName string
	CustURL  string
	Metric   string
	Goal     string
	Cycle    string
}

type tCustomers []*Customer
type tTransactions []*Transaction
type tPushes []*Push
type tPushPrices []*PushPrice
type tFreeTime []*FreeTime

type tIncoming struct {
	data  []*Incoming
	mutex sync.Mutex
}

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

func (f tFreeTime) IndexBefore(t time.Time) int {
	for i := range f {
		if f[i].Date.After(t) {
			return i
		}
	}
	return len(f)
}

func (i *tIncoming) Insert(new *Incoming) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	i.data = append(i.data, new)
	return saveTSV(i.data, "incoming")
}

var customers = tCustomers{}
var transactions = tTransactions{}
var pushes = tPushes{}
var pushprices = tPushPrices{}
var freetime = tFreeTime{}
var incoming = tIncoming{}

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
		Summary: blog.FindEntryByTime(p.Delivered),
	}
}

// --------------------

func tsvPath(table string) string {
	return filepath.Join(dbPath, table+".tsv")
}

func loadTSV(slice interface{}, table string) {
	f, err := os.Open(tsvPath(table))
	// TODO: Unfortunately, this has to compile with Go 1.12 for the time
	// being, so we can't use `errors.Is(err, os.ErrNotExist)` 🙁
	if _, ok := err.(*os.PathError); ok {
		return
	}
	FatalIf(err)
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	FatalIf(gocsv.UnmarshalCSV(reader, slice))
}

func saveTSV(slice interface{}, table string) error {
	fnRegular := tsvPath(table)
	fnNew := fmt.Sprintf("%s-%v.tsv", fnRegular, time.Now().UnixNano())

	f, err := os.Create(fnNew)
	if err != nil {
		return err
	}
	writer := csv.NewWriter(f)
	writer.Comma = '\t'
	err = gocsv.MarshalCSV(slice, gocsv.NewSafeCSVWriter(writer))
	if err != nil {
		return err
	}
	err = f.Close()
	if err != nil {
		return err
	}
	return os.Rename(fnNew, fnRegular)
}

func init() {
	var err error
	var tsvPushes []*pushTSV

	devLocation, err = time.LoadLocation(devLocationName)
	FatalIf(err)

	loadTSV(&customers, "customers")
	loadTSV(&transactions, "transactions")
	loadTSV(&tsvPushes, "pushes")
	loadTSV(&pushprices, "pushprices")
	loadTSV(&freetime, "freetime")
	loadTSV(&incoming.data, "incoming")

	for i := range transactions {
		transactions[i].Outstanding = transactions[i].Cents
	}
	for _, p := range tsvPushes {
		pushes = append(pushes, p.toActualPush())
	}
}
