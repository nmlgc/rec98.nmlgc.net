package main

import (
	"fmt"
	"html/template"
	"log"
	"math"
	"math/big"
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
	ID       ScopedID
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
	Goal               template.HTML
	DelayReason        template.HTML
	PerCustomer        []ContribPerCustomer
	TotalPushFraction  float64
	MissingForFullPush float64
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

	for i := len(transactions.All) - 1; i >= 0; i-- {
		t := transactions.All[i]
		if t.Outstanding.Cmp(&big.Rat{}) > 0 {
			tfg := transactionsForGoal(t.Goal)
			if t.DelayReason != "" {
				if tfg.DelayReason != "" {
					if t.DelayReason != tfg.DelayReason {
						log.Fatalf(`delay reason mismatch at %s:
"%s" (%s) vs.
"%s" (previous)`,
							t.ID, t.DelayReason, t.ID, tfg.DelayReason)
					}
				} else {
					tfg.DelayReason = t.DelayReason
				}
			}
			fpc := tfg.forCustomer(t.Customer)
			fraction, _ := t.Outstanding.Float64()
			opf := OutstandingPushFraction{
				ID:       *t.ID,
				Fraction: fraction,
			}
			fpc.Breakdown = append(fpc.Breakdown, opf)
			fpc.PushFraction += opf.Fraction
			if t.ID.Scope == STransaction {
				tfg.TotalPushFraction += opf.Fraction
			}
		}
	}

	pushPrice := prices.Current().Push
	for i := range ret {
		_, frac := math.Modf(ret[i].TotalPushFraction)
		if frac > 0.0 {
			ret[i].MissingForFullPush = ((1.0 - frac) * pushPrice)
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
		// Assuming that the first diff is the only one that ever touches
		// `master`.
		if len(ret) > 0 && ret[len(ret)-1].DiffInfo.Rev == p.Diff[0].Rev {
			ret[len(ret)-1].Pushes += 1.0
		} else {
			ret = append(ret, DiffInfoWeighted{p.Diff[0], 1.0})
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
	Pushes          int
	PushPrice       float64
	MicroPrice      float64
	Cap             float64
	Outstanding     float64
	Incoming        float64
	Reserved        float64
	FreeEuros       float64
	FracOutstanding float64
	FracIncoming    float64
	FracReserved    float64
	Ctx             any
}

// Reached returns whether the cap has been reached.
func (c Cap) Reached() bool {
	return (c.Outstanding + 1) >= c.Cap
}

// CapCurrent calculates the cap from the current point in time.
func CapCurrent(ctx any) (ret Cap) {
	scopePrices := prices.Current()
	ret.PushPrice = scopePrices.Push
	ret.MicroPrice = scopePrices.Micro

	ret.Pushes = 10
	ret.Cap = float64(ret.Pushes) * ret.PushPrice

	backlog := TransactionBacklog()
	for _, tpg := range backlog {
		if tpg.DelayReason != "" {
			continue
		}
		for _, fpc := range tpg.PerCustomer {
			for _, opf := range fpc.Breakdown {
				switch opf.ID.Scope {
				case STransaction:
					ret.Outstanding += opf.Fraction * ret.PushPrice
				case SMicro:
					ret.Outstanding += opf.Fraction * ret.MicroPrice
				}
			}
		}
	}

	centsIncoming, centsReserved := incoming.Total(
		int(ret.Cap-ret.Outstanding), ret.PushPrice,
	)
	ret.Incoming = float64(centsIncoming)
	ret.Reserved = float64(centsReserved)
	sum := ret.Outstanding + ret.Incoming + ret.Reserved
	fraction := func(dividend, divisor, resultIfNan float64) float64 {
		ret := (dividend / divisor)
		if math.IsNaN(ret) {
			ret = resultIfNan
		}
		return math.Min(ret, 1.0) * 100.0
	}

	ret.FracOutstanding = fraction(ret.Outstanding, ret.Cap, 1.0)
	ret.FracIncoming = fraction(ret.Incoming, ret.Cap, 0.0)
	ret.FracReserved = fraction(ret.Reserved, ret.Cap, 0.0)
	if (ret.FracOutstanding + ret.FracIncoming + ret.FracReserved) > 100.0 {
		ret.FracIncoming = ((100.0 - ret.FracOutstanding) - ret.FracReserved)
	}

	ret.FreeEuros = (ret.Cap - sum) / 100.0
	ret.Ctx = ctx
	return ret
}
