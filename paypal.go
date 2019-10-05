package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/plutov/paypal"
)

var client *paypal.Client

func init() {
	auth := paypal_auth
	if !auth.Initialized() {
		return
	}
	var err error
	client, err = paypal.NewClient(auth.ClientID, auth.Secret, auth.APIBase)
	FatalIf(err)

	_, err = client.GetAccessToken()
	FatalIf(err)
}

type eInvalidAmount struct {
	amount paypal.PurchaseUnitAmount
}

func (e eInvalidAmount) Error() string {
	return fmt.Sprintf("invalid purchase unit amount: %s", e.amount)
}

func parseAmount(amount paypal.PurchaseUnitAmount) (cents int, err error) {
	var euros int
	if amount.Currency != "EUR" {
		return 0, eInvalidAmount{amount}
	}
	n, err := fmt.Sscanf(amount.Value, "%d.%d", &euros, &cents)
	if n != 2 {
		return 0, eInvalidAmount{amount}
	} else if err != nil {
		return 0, err
	}
	return euros*100 + cents, nil
}

// Pulling the amount from PayPal only seems to work for regular orders, not
// subscriptions…
func processOrder(in *Incoming, order *paypal.Order) error {
	in.Time = order.UpdateTime
	for _, pu := range order.PurchaseUnits {
		cents, err := parseAmount(*pu.Amount)
		if err != nil {
			return err
		}
		in.Cents += cents
	}
	return nil
}

func processSubscription(in *Incoming, order *paypal.Order) error {
	in.Time = order.CreateTime
	return nil
}

var transactionIncomingHandler = http.HandlerFunc(
	func(wr http.ResponseWriter, req *http.Request) {
		if req.Method != "POST" {
			http.Redirect(wr, req, "/order", http.StatusSeeOther)
			return
		}
		var in Incoming
		err := json.NewDecoder(req.Body).Decode(&in)
		if err != nil {
			respondWithError(wr, err)
			return
		}

		order, err := client.GetOrder(in.PayPalID)
		if err != nil {
			respondWithError(wr, err)
			return
		}
		switch in.Cycle {
		case "onetime":
			err = processOrder(&in, order)
		case "monthly":
			err = processSubscription(&in, order)
		}
		if err != nil {
			respondWithError(wr, err)
		}
		if err = incoming.Insert(&in); err != nil {
			respondWithError(wr, err)
		}
	},
)
