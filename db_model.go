package main

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
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

// ProjectInfo bundles information on a repository that has been added to.
type ProjectInfo struct {
	Name     string
	BlogTags []string
}

var projectMap = map[string]*ProjectInfo{
	"ReC98":           {"", []string{"rec98"}},
	"rec98.nmlgc.net": {"Website", []string{"website"}},
	"ssg":             {"Seihou", []string{"seihou", "sh01"}},
}

// DiffInfo contains all pieces of information parsed from a GitHub diff URL.
type DiffInfo struct {
	URL     string
	Project *ProjectInfo
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
	project, ok := projectMap[s[1]]
	if !ok {
		fatal("unknown project")
	}
	rev := s[3]

	top, bottom := func() (top *object.Commit, bottom *object.Commit) {
		if project.Name != "" {
			return nil, nil
		}
		switch s[2] {
		case "compare":
			revs := strings.Split(rev, "...")
			if len(revs) == 1 && strings.Contains(rev, "..") {
				fatal("two-dot ranges not supported")
			}
			bottom = must(repo.GetCommit(revs[0]))
			top = must(repo.GetCommit(revs[1]))
		case "commit":
			top = must(repo.GetCommit(rev))
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

func parseID(input string, format *regexp.Regexp) (prefix byte, id int, err error) {
	idStr := format.FindStringSubmatch(input)
	if len(idStr) < 3 {
		return 0, 0, eInvalidID{input}
	}
	ret, err := strconv.ParseUint(idStr[2], 10, 64)
	if err != nil {
		return 0, 0, err
	}
	return idStr[1][0], int(ret), nil
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

type Table interface {
	Name() string
}

type TableRW interface {
	Table
	sync.Locker
}

// TableRWOp runs the given operation on a read-write table, and serializes the
// table back to disk on success.
func TableRWOp[T any](table TableRW, op func() (data []*T, err error)) error {
	table.Lock()
	defer table.Unlock()
	data, err := op()
	if err != nil {
		return err
	}
	return saveTSV(data, table.Name())
}

/// Schemas
/// -------

type IDScope byte

const (
	SMicro       IDScope = 'M'
	SPush        IDScope = 'P'
	STransaction IDScope = 'T'
)

// ToTransaction returns the corresponding transaction scope for the delivery
// scope s.
func (s IDScope) ToTransaction() IDScope {
	switch s {
	case SPush:
		return STransaction
	case SMicro:
		return SMicro
	default:
		log.Fatalf("trying to use %c as a delivery scope?\n", s)
		return 0
	}
}

// CustomerID represents a consecutively numbered, 1-based customer ID.
type CustomerID int

// ScopedID represents a consecutively numbered, 1-based ID in any of the ID
// scopes.
type ScopedID struct {
	Scope IDScope
	ID    int
}

var scopedIDFormat = regexp.MustCompile("(M|P|T)([0-9]{4})")

func (i ScopedID) String() string {
	return fmt.Sprintf("%c%04d", i.Scope, i.ID)
}

// UnmarshalCSV decodes a ScopedID from its string representation.
func (i *ScopedID) UnmarshalCSV(s string) error {
	prefix, id, err := parseID(s, scopedIDFormat)
	*i = ScopedID{ID: id, Scope: IDScope(prefix)}
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
	ID       *ScopedID
	Time     time.Time
	Customer CustomerID
	Cents    int
	Goal     template.HTML
	Delayed  bool

	// Calculated after the push table has loaded
	Outstanding big.Rat
}

// Consumes outstanding cents up to the remaining fraction from p, and returns
// the new remaining push fraction.
func (t *Transaction) consume(p *pushTSV, fractionNeeded *big.Rat) *big.Rat {
	if t.ID.Scope == SMicro {
		t.Outstanding.SetInt64(0)
		return fractionNeeded.SetInt64(0)
	}

	nullRat := &big.Rat{}
	if fractionNeeded.Cmp(nullRat) <= 0 {
		log.Fatalf(
			"%s consumed more transactions than it should have (%v)",
			p.ID, p.Transactions,
		)
	} else if t.Outstanding.Cmp(nullRat) <= 0 {
		log.Fatalf("more pushes associated with %s than it paid for", t.ID)
	}

	t.Outstanding.Sub(&t.Outstanding, fractionNeeded)

	// Did we need more than this transaction paid for?
	if t.Outstanding.Cmp(nullRat) == -1 {
		// If yes, we fully consumed this transaction. We still need the amount
		// that's now in the negative.
		fractionNeeded = fractionNeeded.Set(&t.Outstanding).Neg(fractionNeeded)
		t.Outstanding.SetInt64(0)
		return fractionNeeded
	}

	// Got the exact fraction.
	return fractionNeeded.SetInt64(0)
}

// Push represents a single unit of work.
type Push struct {
	ID                ScopedID
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
	CustName string
	CustURL  string
	Metric   string
	Goal     string
	Cycle    string
	Micro    bool
	// 1-based index into the discountOffers array, or 0 for none.
	Discount DiscountID
	// Retrieved from PayPal
	Cents int
	Time  *time.Time
	// Will only render the associated discount reserve as part of the cap.
	ConfirmedAndWaitingForDiscount bool

	// Session ID assigned by the payment provider
	ProviderSession string
}

// TagDescription bundles a blog tag with a descriptive sentence.
type TagDescription struct {
	Tag  string
	Desc string
}

// ProviderAuth collects all data required for authenticating with payment
// providers.
type ProviderAuth struct {
	APIBase  string
	ClientID string
	Secret   string
}

// StripeSub contains the key to cancel a Stripe subscription.
type StripeSub struct {
	Salt string
	ID   string
}

type tCustomers []*Customer
type tTransactions struct {
	All    []*Transaction
	Scoped map[IDScope][]*Transaction
}
type tPushes []*Push
type tPushPrices []*PushPrice
type tFreeTime []*FreeTime
type tBlogTags map[string][]string
type tTagDescriptions struct {
	Ordered []*TagDescription
	Map     map[string]string
}

type tIncoming struct {
	sync.Mutex
	data []*Incoming
}

func (t *tIncoming) Name() string { return "incoming" }

type tStripeSubs struct {
	sync.Mutex
	data map[string]string
}

func (t *tStripeSubs) Name() string { return "stripe_subs" }

func (c tCustomers) ByID(id CustomerID) Customer {
	return *c[id]
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

// Total calculates the total amount of incoming and reserved cents, with
// discount round-ups being calculated relative to the given remaining amount
// of money in the cap.
func (i *tIncoming) Total(capRemaining int, pushprice float64) (cents int, reserved int) {
	for _, in := range i.data {
		if !in.ConfirmedAndWaitingForDiscount {
			cents += in.Cents
			capRemaining -= in.Cents
		}
		if in.Discount != 0 {
			offer := discountOffers[in.Discount-1]
			fraction := offer.FractionCovered(pushprice)
			roundupValue := int(DiscountRoundupValue(
				float64(capRemaining), float64(in.Cents), pushprice, fraction,
			))
			reserved += roundupValue
			capRemaining -= roundupValue
		}
	}
	return
}

type eIncomingInsertError struct{}

func (e eIncomingInsertError) Error() string {
	return "malformed transaction"
}

func (i *tIncoming) Insert(new *Incoming) error {
	return TableRWOp(&incoming, func() ([]*Incoming, error) {
		// No timestamp?
		if new.Time == nil {
			return nil, eIncomingInsertError{}
		}
		for oldIn := range i.data {
			// Duplicates?
			if i.data[oldIn].ProviderSession == new.ProviderSession {
				return nil, eIncomingInsertError{}
			}
		}
		i.data = append(i.data, new)
		return i.data, nil
	})
}

func (t *tStripeSubs) ToSlice() (ret []*StripeSub) {
	for salt, id := range t.data {
		ret = append(ret, &StripeSub{Salt: salt, ID: id})
	}
	return
}

func (t *tStripeSubs) Insert(salt string, id string) error {
	return TableRWOp(&stripeSubs, func() ([]*StripeSub, error) {
		t.data[salt] = id
		return t.ToSlice(), nil
	})
}

func (t *tStripeSubs) Delete(salt string) error {
	return TableRWOp(&stripeSubs, func() ([]*StripeSub, error) {
		delete(t.data, salt)
		return t.ToSlice(), nil
	})
}

var customers = tCustomers{}
var transactions = tTransactions{}
var pushprices = tPushPrices{}
var freetime = tFreeTime{}
var incoming = tIncoming{}
var blogTags = tBlogTags{}
var tagDescriptions = tTagDescriptions{}
var providerAuth = make(map[string]ProviderAuth)
var stripeSubs = tStripeSubs{data: make(map[string]string)}

/// -------

// TSV input structures
// --------------------
type pushTSV struct {
	ID                *ScopedID
	Transactions      []int
	Goal              string
	Delivered         time.Time
	Diff              string
	IncludeInEstimate bool
}

func (p *pushTSV) toActualPush(repo *Repository) *Push {
	return &Push{
		ID:                *p.ID,
		Goal:              p.Goal,
		Delivered:         p.Delivered,
		Diff:              NewDiffInfo(p.Diff, repo),
		IncludeInEstimate: p.IncludeInEstimate,

		Transactions: func() (ret []*Transaction) {
			if len(p.Transactions) == 0 {
				log.Fatalf("%s has no transactions associated with it", p.ID)
			}
			fractionNeeded := big.NewRat(1, 1)
			for _, tid := range p.Transactions {
				t := transactions.Scoped[p.ID.Scope.ToTransaction()][tid-1]
				ret = append(ret, t)
			}
			for _, t := range ret {
				fractionNeeded = t.consume(p, fractionNeeded)
			}
			if fractionNeeded.Cmp(&big.Rat{}) != 0 {
				log.Fatalf(
					"%s is not fully paid for (missing %v pushes)",
					p.ID, fractionNeeded,
				)
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
	for _, p := range tsvPushes {
		ret = append(ret, p.toActualPush(repo))
	}
	return
}

type providerAuthTSV struct {
	Provider string
	ProviderAuth
}

// --------------------

func tsvPath(table string) string {
	return filepath.Join(dbPath, table+".tsv")
}

func loadTSV(slice interface{}, table string, unmarshaler func(gocsv.CSVReader, interface{}) error) {
	f, err := os.Open(tsvPath(table))
	// TODO: Unfortunately, this has to compile with Go 1.12 for the time
	// being, so we can't use `errors.Is(err, os.ErrNotExist)` 🙁
	if _, ok := err.(*os.PathError); ok {
		return
	}
	FatalIf(err)
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.LazyQuotes = true
	FatalIf(unmarshaler(reader, slice))
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
	var providerAuths []*providerAuthTSV

	devLocation, err = time.LoadLocation(devLocationName)
	FatalIf(err)

	loadTSV(&customers, "customers", gocsv.UnmarshalCSV)
	loadTSV(&transactions.All, "transactions", gocsv.UnmarshalCSV)
	loadTSV(&tsvPushes, "pushes", gocsv.UnmarshalCSV)
	loadTSV(&pushprices, "pushprices", gocsv.UnmarshalCSV)
	loadTSV(&freetime, "freetime", gocsv.UnmarshalCSV)
	loadTSV(&incoming.data, incoming.Name(), gocsv.UnmarshalCSV)
	loadTSV(&blogTags, "blog_tags", gocsv.UnmarshalCSVToMap)
	loadTSV(&tagDescriptions.Ordered, "tag_descriptions", gocsv.UnmarshalCSV)
	loadTSV(&providerAuths, "provider_auth", gocsv.UnmarshalCSV)
	loadTSV(&stripeSubs.data, stripeSubs.Name(), gocsv.UnmarshalCSVToMap)

	transactions.Scoped = make(map[IDScope][]*Transaction)
	for _, transaction := range transactions.All {
		pushprice := int64(pushprices.At(transaction.Time))
		transaction.Outstanding.SetFrac64(int64(transaction.Cents), pushprice)
		transactions.Scoped[transaction.ID.Scope] = append(
			transactions.Scoped[transaction.ID.Scope], transaction,
		)
	}

	tagDescriptions.Map = make(map[string]string)
	for _, td := range tagDescriptions.Ordered {
		tagDescriptions.Map[td.Tag] = td.Desc
	}

	for _, auth := range providerAuths {
		providerAuth[auth.Provider] = auth.ProviderAuth
	}
}
