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

	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/gocarina/gocsv"
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
	Top     *object.Commit // Can be nil if not belonging to ReC98
	Bottom  *object.Commit // Can be nil if not belonging to ReC98
}

// NewDiffInfo parses a GitHub diff URL into a DiffInfo structure, resolving
// its top and bottom commits using the given repo.
func NewDiffInfo(url string, repo *Repository) DiffInfo {
	fatal := func(err string) {
		log.Fatalf("%s: %s\n", url, err)
	}
	must := func(ret *object.Commit, err error) *object.Commit {
		if err != nil {
			fatal(err.Error())
		}
		return ret
	}

	if len(url) == 0 {
		fatal("no diff URL provided")
	}
	s := strings.Split(url, "/")
	if len(s) != 4 {
		fatal("expected 3 slashes")
	}
	project := ""
	if s[1] == "rec98.nmlgc.net" {
		project = "Website"
	}
	rev := s[3]

	top, bottom := func() (top *object.Commit, bottom *object.Commit) {
		if project != "" {
			return nil, nil
		}
		switch s[2] {
		case "compare":
			revs := strings.Split(rev, "...")
			if len(revs) == 1 && strings.Contains(rev, "..") {
				fatal("two-dot ranges not supported")
			}
			bottom = must(getCommit(revs[0]))
			top = must(getCommit(revs[1]))
		case "commit":
			top = must(getCommit(rev))
			if len(top.ParentHashes) > 1 {
				fatal("more than one parent; use the \"compare\" mode instead!")
			}
			bottom = must(top.Parent(0))
		default:
			fatal("mode must be either \"compare\" or \"commit\"")
		}
		return
	}()
	return DiffInfo{
		URL:     url,
		Project: project,
		Rev:     rev,
		Top:     top,
		Bottom:  bottom,
	}
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

// DateInDevLocation decodes an ISO 8601 date to a LocalDateStamp.
func DateInDevLocation(s string) (ret LocalDateStamp) {
	FatalIf(ret.UnmarshalCSV(s))
	return
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

// PushID represents a consecutively numbered, 1-based push ID.
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
	Diff              DiffInfo
	IncludeInEstimate bool
}

// FundedBy returns all customers that were involved in funding this push.
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
	// Retrieved via the POST body
	PayPalID string
	CustName string
	CustURL  string
	Metric   string
	Goal     string
	Cycle    string
	// Retrieved from PayPal
	Cents int
	Time  *time.Time
}

// PayPalAuth collects all data required for authenticating with PayPal.
type PayPalAuth struct {
	APIBase  string
	ClientID string
	Secret   string
}

// Initialized returns whether we have PayPal credentials.
func (p PayPalAuth) Initialized() bool {
	return p.APIBase != ""
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
	return *c[id]
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

func (i *tIncoming) Total() (cents int) {
	for _, in := range i.data {
		cents += in.Cents
	}
	return
}

type eIncomingInsertError struct{}

func (e eIncomingInsertError) Error() string {
	return "malformed transaction"
}

func (i *tIncoming) Insert(new *Incoming) error {
	i.mutex.Lock()
	defer i.mutex.Unlock()
	// No timestamp?
	if new.Time == nil {
		return eIncomingInsertError{}
	}
	for oldIn := range i.data {
		// Duplicates?
		if i.data[oldIn].PayPalID == new.PayPalID {
			return eIncomingInsertError{}
		}
	}
	i.data = append(i.data, new)
	return saveTSV(i.data, "incoming")
}

var customers = tCustomers{}
var transactions = tTransactions{}
var pushprices = tPushPrices{}
var freetime = tFreeTime{}
var incoming = tIncoming{}
var paypal_auth = PayPalAuth{}

/// -------

// TSV input structures
// --------------------
type pushTSV struct {
	ID                PushID
	Transactions      []TransactionID
	Goal              string
	Delivered         time.Time
	Diff              string
	IncludeInEstimate bool
}

func (p *pushTSV) toActualPush(repo *Repository) *Push {
	return &Push{
		ID:                p.ID,
		Goal:              p.Goal,
		Delivered:         p.Delivered,
		Diff:              NewDiffInfo(p.Diff, repo),
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
	}
}

var tsvPushes []*pushTSV

// NewPushes parses tsv into a tPushes object, consuming the given transactions
// and validating their assignment to the respective pushes. Commit references
// are directly resolved using the given repo.
func NewPushes(transactions tTransactions, tsv []*pushTSV, repo *Repository) (ret tPushes) {
	for i := range transactions {
		transactions[i].Outstanding = transactions[i].Cents
	}
	for _, p := range tsvPushes {
		ret = append(ret, p.toActualPush(repo))
	}
	return
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
	var paypalAuths []*PayPalAuth

	devLocation, err = time.LoadLocation(devLocationName)
	FatalIf(err)

	loadTSV(&customers, "customers")
	loadTSV(&transactions, "transactions")
	loadTSV(&tsvPushes, "pushes")
	loadTSV(&pushprices, "pushprices")
	loadTSV(&freetime, "freetime")
	loadTSV(&incoming.data, "incoming")
	loadTSV(&paypalAuths, "paypal_auth")

	if len(paypalAuths) > 0 {
		paypal_auth = *paypalAuths[0]
		log.Println("Using PayPal auth", paypal_auth)
	} else {
		log.Println("paypal_auth table is empty, disabling integration")
	}
}
