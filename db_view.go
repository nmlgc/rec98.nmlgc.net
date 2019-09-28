package main

import (
	"fmt"
	"html/template"
	"log"
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
	if err != nil {
		log.Fatalln(err)
	}
	aY, aM, aD := dp.Date()
	return PushesWhere(func(p Push) bool {
		pY, pM, pD := p.Delivered.Date()
		return pD == aD && pM == aM && pY == aY
	})
}
