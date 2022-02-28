package main

import (
	"fmt"
	"html/template"
	"math"
	"strconv"
)

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

type eDiscountIDOutOfRange struct {
	ID uint
}

func (e eDiscountIDOutOfRange) Error() string {
	return fmt.Sprintf("discount ID out of range: %d", e.ID)
}

// DiscountID represents a 1-based index into discountOffers, or 0 for no
// discount.
type DiscountID uint

func NewDiscountID(s string) (DiscountID, error) {
	parsed, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, err
	}
	if int(parsed) > len(discountOffers) {
		return 0, eDiscountIDOutOfRange{uint(parsed)}
	}
	return DiscountID(uint(parsed)), nil
}

// UnmarshalCSV decodes a DiscountID from its CSV string representation.
func (i *DiscountID) UnmarshalCSV(s string) (err error) {
	*i, err = NewDiscountID(s)
	return err
}

// UnmarshalJSON decodes a DiscountID from its JSON representation.
func (i *DiscountID) UnmarshalJSON(s []byte) (err error) {
	*i, err = NewDiscountID(string(s))
	return err
}

// Calculates the amount of money rounded up by a sponsor for a transaction
// with the given amount, limited to the cap.
// Must match the implementation in static/paypal.js!
func DiscountRoundupValue(capRemainingAfterAmount, amount, pushprice, discountFraction float64) float64 {
	pushpriceDiscounted := (pushprice * (1 - discountFraction))
	roundupValue := (pushprice - pushpriceDiscounted)
	return math.RoundToEven(math.Min(
		((amount / pushpriceDiscounted) * roundupValue),
		capRemainingAfterAmount,
	))
}
