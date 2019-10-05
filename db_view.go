package main

import (
	"fmt"
	"html/template"
	"math"
	"sort"
	"time"
)

// CustomerByID returns a HTML representation of the given customer.
func CustomerByID(id CustomerID) template.HTML {
	c := customers.ByID(id)
	if len(c.URL) == 0 {
		return template.HTML(c.Name)
	}
	return template.HTML(fmt.Sprintf(
		`<a class="customer" href="%s">%s</a>`, c.URL, c.Name,
	))
}

// ContribPerCustomer represents the contribution of a single customer towards
// the goal this structure is included in.
type ContribPerCustomer struct {
	Customer     CustomerID
	PushFraction float64
}

// TransactionsPerGoal lists all contributions towards a specific goal.
type TransactionsPerGoal struct {
	Goal        string
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
	transactionsForGoal := func(goal string) *TransactionsPerGoal {
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
			fpc := tfg.forCustomer(t.Customer)
			pushprice := pushprices.At(t.Time)
			fpc.PushFraction += float64(t.Outstanding) / float64(pushprice)
		}
	}
	return ret
}

// PushesWhere returns the pushes fulfilling the given predicate, with the
// latest pushes at the beginning.
func PushesWhere(pred func(p Push) bool) (ret []Push) {
	for i := len(pushes) - 1; i >= 0; i-- {
		p := *pushes[i]
		if pred(p) {
			ret = append(ret, p)
		}
	}
	return ret
}

// Pushes returns all pushes, with the latest deliveries at the beginning.
func Pushes() []Push {
	return PushesWhere(func(p Push) bool { return true })
}

// DiffInfoWeighted combines a DiffInfo with the number of pushes it took to
// get done.
type DiffInfoWeighted struct {
	DiffInfo
	Pushes float64
}

// DiffsForEstimate returns the diffs of all pushes that are part of the
// progress estimate, sorted from the latest to the earliest delivery.
func DiffsForEstimate() (ret []DiffInfoWeighted) {
	selected := PushesWhere(func(p Push) bool {
		return p.IncludeInEstimate
	})
	sort.SliceStable(selected, func(i, j int) bool {
		return selected[i].Delivered.After(selected[j].Delivered)
	})
	for _, p := range selected {
		if len(ret) > 0 && ret[len(ret)-1].DiffInfo == *p.Diff {
			ret[len(ret)-1].Pushes += 1.0
		} else {
			ret = append(ret, DiffInfoWeighted{*p.Diff, 1.0})
		}
	}
	return
}

// PushesDeliveredAt returns all pushes delivered at the given date.
func PushesDeliveredAt(datestring string) []Push {
	dp, err := time.Parse("2006-01-02", datestring)
	FatalIf(err)
	aY, aM, aD := dp.Date()
	return PushesWhere(func(p Push) bool {
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
	Cap             float64
	Outstanding     float64
	Incoming        float64
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
	price := pushprices.Current()
	ret.Now = time.Now()
	ret.Then = ret.Now.Add(CapWindow)

	start := freetime.IndexBefore(ret.Now)
	end := freetime.IndexBefore(ret.Then)

	ret.Pushes = end - start
	ret.Cap = float64(ret.Pushes) * price

	backlog := TransactionBacklog()
	for _, tpg := range backlog {
		// Yes, a very ugly hack...
		if tpg.Goal == "ReC98 having a good future" {
			continue
		}
		for _, fpc := range tpg.PerCustomer {
			ret.Outstanding += fpc.PushFraction * price
		}
	}

	ret.Incoming = float64(incoming.Total())
	sum := ret.Outstanding + ret.Incoming
	if sum >= ret.Cap {
		firstfree := start + int(sum/price)
		if firstfree < len(freetime) {
			ret.FirstFree = &freetime[firstfree].Date.Time
		}
	}

	ret.FracOutstanding = math.Min(ret.Outstanding/ret.Cap, 1.0) * 100.0
	ret.FracIncoming = math.Min((ret.Incoming/ret.Cap), 1.0) * 100.0
	if (ret.FracOutstanding + ret.FracIncoming) > 100.0 {
		ret.FracIncoming = 100.0 - ret.FracOutstanding
	}
	ret.Ctx = ctx
	return ret
}
