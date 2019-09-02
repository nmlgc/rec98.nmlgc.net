package main

import (
	"html/template"
)

// CustomerByID returns a HTML representation of the given customer.
func CustomerByID(id CustomerID) template.HTML {
	return template.HTML(customers.ByID(id).Name)
}

// PushProjection indicates the various types of pushes for push_table.html.
type PushProjection struct {
	IsDelivered bool
	Rows        []Push // with the latest pushes at the beginning
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

// PushesOutstanding returns pushes that have not been delivered yet.
func PushesOutstanding() PushProjection {
	return PushProjection{
		false,
		PushesWhere(func(p Push) bool { return p.Delivered.Time == nil }),
	}
}

// PushesDelivered returns completed pushes.
func PushesDelivered() PushProjection {
	return PushProjection{
		true,
		PushesWhere(func(p Push) bool { return p.Delivered.Time != nil }),
	}
}
