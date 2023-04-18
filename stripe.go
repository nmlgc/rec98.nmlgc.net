package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"
)

const STRIPE_SESSION_CACHE = "stripe_sessions.gob"

type StripeClient struct {
	URLSuccess       string
	URLCancel        string
	RouteAPIIncoming string
	RouteAPISuccess  string
	sync.Mutex
}

func NewStripeClient(domain *url.URL, apiPrefix string) *StripeClient {
	auth, ok := providerAuth["stripe"]
	if !ok {
		return nil
	}
	stripe.Key = auth.Secret

	routeAPISuccess := apiPrefix + "/success"
	urlSuccess := domain.JoinPath(routeAPISuccess)
	urlCancel := domain.JoinPath("/order")
	urlSuccess.RawQuery = "session_id={CHECKOUT_SESSION_ID}"

	log.Println("Using Stripe auth", auth.Secret)
	return &StripeClient{
		URLSuccess:       urlSuccess.String(),
		URLCancel:        urlCancel.String(),
		RouteAPIIncoming: (apiPrefix + "/incoming"),
		RouteAPISuccess:  routeAPISuccess,
	}
}

func (c *StripeClient) HandleIncoming(wr http.ResponseWriter, req *http.Request, in *Incoming) {
	str := stripe.String
	i64 := stripe.Int64

	params := &stripe.CheckoutSessionParams{
		SuccessURL: &c.URLSuccess,
		CancelURL:  &c.URLCancel,
		LineItems: []*stripe.CheckoutSessionLineItemParams{{
			PriceData: &stripe.CheckoutSessionLineItemPriceDataParams{
				Currency:    str(string(stripe.CurrencyEUR)),
				Product:     str("prod_NiZac7KHxgcQul"),
				UnitAmount:  i64(int64(in.Cents)),
				TaxBehavior: str(string(stripe.PriceTaxBehaviorInclusive)),
			},
			Quantity: i64(1),
		}},
	}
	if in.Cycle == "monthly" {
		params.Mode = str(string(stripe.CheckoutSessionModeSubscription))
		params.LineItems[0].PriceData.Recurring = &stripe.CheckoutSessionLineItemPriceDataRecurringParams{
			Interval:      str("month"),
			IntervalCount: i64(1),
		}
	} else {
		params.Mode = str(string(stripe.CheckoutSessionModePayment))
	}

	// Just in case the server crashes between here and the success handler…
	log.Println("Incoming Stripe payment request:", *in)

	s, err := session.New(params)
	if err != nil {
		respondWithError(wr, err)
		return
	}
	time := time.Unix(s.Created, 0)
	in.ProviderSession = s.ID
	in.Time = &time

	c.Lock()
	defer c.Unlock()
	sessions, err := CacheLoad[map[string]*Incoming](STRIPE_SESSION_CACHE)
	if err != nil {
		sessions = make(map[string]*Incoming)
	}
	sessions[s.ID] = in
	CacheSave(STRIPE_SESSION_CACHE, sessions)

	// https://github.com/whatwg/fetch/issues/763#issuecomment-1430650132
	wr.Header().Add("Location", s.URL)
	wr.WriteHeader(http.StatusNoContent)
}

func (c *StripeClient) HandleSuccess(wr http.ResponseWriter, req *http.Request) {
	sessionID := req.URL.Query().Get("session_id")

	c.Lock()
	defer c.Unlock()
	sessions, err := CacheLoad[map[string]*Incoming](STRIPE_SESSION_CACHE)
	if err != nil {
		//lint:ignore ST1005 People might read this one.
		respondWithError(wr, fmt.Errorf(
			"Failed to load Stripe session cache?! I should have received your order though. If I did, you will soon receive a confirmation email.",
		))
		return
	}

	in, ok := sessions[sessionID]
	if !ok {
		respondWithError(wr, fmt.Errorf(
			"invalid Stripe session ID: %v", sessionID,
		))
		return
	}

	s, err := session.Get(sessionID, nil)
	if err != nil {
		respondWithError(wr, err)
		return
	}

	if (s.Status != stripe.CheckoutSessionStatusComplete) ||
		(s.PaymentStatus != stripe.CheckoutSessionPaymentStatusPaid) {
		wr.WriteHeader(http.StatusPaymentRequired)
		fmt.Fprintln(wr, "Nice try.")
		return
	}

	if err := incoming.Insert(in); err != nil {
		respondWithError(wr, err)
		return
	}
	delete(sessions, sessionID)
	CacheSave(STRIPE_SESSION_CACHE, sessions)
	http.Redirect(wr, req, "/thankyou", http.StatusSeeOther)
}
