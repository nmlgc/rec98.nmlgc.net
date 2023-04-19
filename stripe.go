package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/stripe/stripe-go/v74"
	"github.com/stripe/stripe-go/v74/checkout/session"
	"github.com/stripe/stripe-go/v74/subscription"
)

const STRIPE_SESSION_CACHE = "stripe_sessions.gob"

type StripeClient struct {
	URLSuccess       string
	URLCancel        string
	RouteAPIIncoming string
	RouteAPISuccess  string
	RouteAPICancel   string
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
		RouteAPICancel:   (apiPrefix + "/cancel"),
	}
}

func stripeSaltedSubID(salt string, subID string) string {
	hash := CryptHashOfSlice([]byte(subID + salt))
	ret := base64.StdEncoding.EncodeToString(hash[:])
	return ret
}

// Sub accesses encrypted subscription data.
func (c *StripeClient) Sub(salt string) (string, error) {
	saltedSubID, ok := stripeSubs.data[salt]
	if !ok {
		return "", errors.New("unknown subscription")
	}
	return saltedSubID, nil
}

func (c *StripeClient) subIDVerify(salt string, subID string) error {
	expected, err := c.Sub(salt)
	if err != nil {
		return err
	}
	if expected != stripeSaltedSubID(salt, subID) {
		return errors.New("unknown combination of salt and subscription ID")
	}
	return nil
}

func (c *StripeClient) subDataFromForm(req *http.Request) (salt string, subID string, err error) {
	if err := req.ParseForm(); err != nil {
		return "", "", err
	}
	salt = strings.TrimSpace(req.PostForm.Get("salt"))
	subID = strings.TrimSpace(req.PostForm.Get("sub_id"))
	if (len(salt) == 0) || (len(subID) == 0) {
		return "", "", errors.New("missing form data")
	}
	return salt, subID, c.subIDVerify(salt, subID)
}

type stripeSessionView struct {
	SubID string
}

func (c *StripeClient) Session(sessionID string, salt string) (*stripeSessionView, error) {
	s, err := session.Get(sessionID, nil)
	if err != nil {
		return nil, err
	}
	if err := c.subIDVerify(salt, s.Subscription.ID); err != nil {
		return nil, err
	}
	return &stripeSessionView{
		SubID: s.Subscription.ID,
	}, nil
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

	if s.Subscription != nil {
		saltBytes := make([]byte, 8)
		salt := ""
		ok = true
		for ok {
			_, err := rand.Read(saltBytes)
			if err != nil {
				respondWithError(wr, fmt.Errorf(
					"error generating cancellation key for subscription: %v",
					err,
				))
				return
			}
			salt = base64.URLEncoding.EncodeToString(saltBytes)
			_, ok = stripeSubs.data[salt]
		}
		stripeSubs.Insert(salt, stripeSaltedSubID(salt, s.Subscription.ID))
	}

	delete(sessions, sessionID)
	CacheSave(STRIPE_SESSION_CACHE, sessions)
	http.Redirect(wr, req, "/thankyou", http.StatusSeeOther)
}

func (c *StripeClient) HandleCancel(wr http.ResponseWriter, req *http.Request) {
	salt, subID, err := c.subDataFromForm(req)
	if err != nil {
		respondWithError(wr, err)
		return
	}
	if _, err := subscription.Cancel(subID, nil); err != nil {
		respondWithError(wr, err)
		return
	}
	if err := stripeSubs.Delete(salt); err != nil {
		//lint:ignore ST1005 People might read this one.
		respondWithError(wr, fmt.Errorf(
			"Failed to remove the subscription from the server: %v. It was properly canceled though.",
			err,
		))
	}
	wr.WriteHeader(http.StatusNoContent)
}
