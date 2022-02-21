package main

import "html/template"

// DiscountOffer represents an offer by a customer to financially support a
// given goal by covering a part of every push purchased by other customers.
// Hardcoded because I'd *really* like to use special formatting in the goal
// strings.
type DiscountOffer struct {
	Sponsor      CustomerID
	CentsCovered int
	Goals        []template.HTML
	Ad           template.HTML
}

func (o *DiscountOffer) FractionCovered(pushprice float64) float64 {
	return (float64(o.CentsCovered) / pushprice)
}

type DiscountOfferView struct {
	DiscountOffer
	FractionCovered    float64
	PushpriceRemaining float64
}

var discountOffers = []DiscountOffer{
	{Sponsor: 2, CentsCovered: 2000, Goals: []template.HTML{
		HTMLEmoji("th05") + " TH05 reverse-engineering",
		HTMLEmoji("th05") + " TH05 replay support",
		"C89 conformance (any game)",
		"Compatibility with pre-386 PC-98 models (any game)",
	}, Ad: HTMLEmoji("th05") + " TH05 danmaku scripting toolkit release planned around 50% RE of <code>MAIN.EXE</code>!"},
}

func DiscountOffers(pushprice float64) (ret []DiscountOfferView) {
	for _, offer := range discountOffers {
		cents := float64(offer.CentsCovered)
		ret = append(ret, DiscountOfferView{
			DiscountOffer:      offer,
			FractionCovered:    offer.FractionCovered(pushprice),
			PushpriceRemaining: (pushprice - cents),
		})
	}
	return
}
