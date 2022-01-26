package main

import (
	"fmt"
	"html/template"
	"math"
	"sort"
	"time"
)

// HTMLByID returns a HTML representation of the given customer.
func (t tCustomers) HTMLByID(id CustomerID) template.HTML {
	c := t.ByID(id)
	if len(c.URL) == 0 {
		return template.HTML(c.Name)
	}
	return template.HTML(fmt.Sprintf(
		`<a class="customer" href="%s">%s</a>`, c.URL, c.Name,
	))
}

// OutstandingPushFraction bundles a transaction ID with its outstanding
// amount of pushes.
type OutstandingPushFraction struct {
	ID       TransactionID
	Fraction float64
}

// ContribPerCustomer represents the contribution of a single customer towards
// the goal this structure is included in.
type ContribPerCustomer struct {
	Customer     CustomerID
	PushFraction float64
	Breakdown    []OutstandingPushFraction
}

// TransactionsPerGoal lists all contributions towards a specific goal.
type TransactionsPerGoal struct {
	Goal        template.HTML
	Delayed     bool
	PerCustomer []ContribPerCustomer
}

func (t *TransactionsPerGoal) forCustomer(c CustomerID) *ContribPerCustomer {
	for i := range t.PerCustomer {
		if t.PerCustomer[i].Customer == c {
			return &t.PerCustomer[i]
		}
	}
	t.PerCustomer = append(t.PerCustomer, ContribPerCustomer{Customer: c})
	return &t.PerCustomer[len(t.PerCustomer)-1]
}

// TransactionBacklog returns all transactions that haven't been consumed by
// pushes, sorted by goal and customer.
func TransactionBacklog() (ret []TransactionsPerGoal) {
	transactionsForGoal := func(goal template.HTML) *TransactionsPerGoal {
		for i := range ret {
			if ret[i].Goal == goal {
				return &ret[i]
			}
		}
		ret = append(ret, TransactionsPerGoal{Goal: goal})
		return &ret[len(ret)-1]
	}

	for i := len(transactions) - 1; i >= 0; i-- {
		t := transactions[i]
		if t.Outstanding > 0 {
			tfg := transactionsForGoal(t.Goal)
			tfg.Delayed = tfg.Delayed || t.Delayed
			fpc := tfg.forCustomer(t.Customer)
			pushprice := pushprices.At(t.Time)
			opf := OutstandingPushFraction{
				ID:       t.ID,
				Fraction: float64(t.Outstanding) / float64(pushprice),
			}
			fpc.Breakdown = append(fpc.Breakdown, opf)
			fpc.PushFraction += opf.Fraction
		}
	}
	return ret
}

// Where returns the pushes fulfilling the given predicate, with the
// latest pushes at the beginning.
func (t tPushes) Where(pred func(p Push) bool) (ret []Push) {
	for i := len(t) - 1; i >= 0; i-- {
		p := *t[i]
		if pred(p) {
			ret = append(ret, p)
		}
	}
	return ret
}

// All returns all pushes, with the latest deliveries at the beginning.
func (t tPushes) All() []Push {
	return t.Where(func(p Push) bool { return true })
}

// DiffInfoWeighted combines a DiffInfo with the number of pushes it took to
// get done.
type DiffInfoWeighted struct {
	DiffInfo
	Pushes float64
}

// DiffsForEstimate returns the diffs of all pushes that are part of the
// progress estimate, sorted from the latest to the earliest delivery.
func (t tPushes) DiffsForEstimate() (ret []DiffInfoWeighted) {
	selected := t.Where(func(p Push) bool {
		return p.IncludeInEstimate
	})
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Delivered.After(selected[j].Delivered)
	})
	for _, p := range selected {
		if len(ret) > 0 && ret[len(ret)-1].DiffInfo.Rev == p.Diff.Rev {
			ret[len(ret)-1].Pushes += 1.0
		} else {
			ret = append(ret, DiffInfoWeighted{p.Diff, 1.0})
		}
	}
	return
}

// DeliveredAt returns all pushes delivered at the given date.
func (t tPushes) DeliveredAt(datestring string) []Push {
	dp, err := time.Parse("2006-01-02", datestring)
	FatalIf(err)
	aY, aM, aD := dp.Date()
	return t.Where(func(p Push) bool {
		pY, pM, pD := p.Delivered.Date()
		return pD == aD && pM == aM && pY == aY
	})
}

// Cap bundles all information about the current state of the backlog, with
// regard to the crowdfunding cap.
type Cap struct {
	Now             time.Time
	Then            time.Time
	FirstFree       *time.Time
	Pushes          int
	PushPrice       float64
	Cap             float64
	Outstanding     float64
	Incoming        float64
	FreeEuros       float64
	FracOutstanding float64
	FracIncoming    float64
	Ctx             interface{}
}

// Reached returns whether the cap has been reached.
func (c Cap) Reached() bool {
	return c.Outstanding >= c.Cap
}

// CapCurrent calculates the cap from the current point in time.
func CapCurrent(ctx interface{}) (ret Cap) {
	ret.PushPrice = pushprices.Current()
	ret.Now = time.Now()
	ret.Then = ret.Now.Add(CapWindow)

	start := freetime.IndexBefore(ret.Now)
	end := freetime.IndexBefore(ret.Then)

	ret.Pushes = end - start
	ret.Cap = float64(ret.Pushes) * ret.PushPrice

	backlog := TransactionBacklog()
	for _, tpg := range backlog {
		if tpg.Delayed {
			continue
		}
		for _, fpc := range tpg.PerCustomer {
			ret.Outstanding += fpc.PushFraction * ret.PushPrice
		}
	}

	ret.Incoming = float64(incoming.Total())
	sum := ret.Outstanding + ret.Incoming
	if sum >= ret.Cap {
		firstfree := start + int(sum/ret.PushPrice)
		if firstfree < len(freetime) {
			ret.FirstFree = &freetime[firstfree].Date.Time
		}
	}

	fraction := func(dividend, divisor float64) float64 {
		ret := (dividend / divisor)
		if math.IsNaN(ret) {
			ret = 1.0
		}
		return math.Min(ret, 1.0) * 100.0
	}

	ret.FracOutstanding = fraction(ret.Outstanding, ret.Cap)
	ret.FracIncoming = fraction(ret.Incoming, ret.Cap)
	if (ret.FracOutstanding + ret.FracIncoming) > 100.0 {
		ret.FracIncoming = 100.0 - ret.FracOutstanding
	}

	ret.FreeEuros = (ret.Cap - sum) / 100.0
	ret.Ctx = ctx
	return ret
}
