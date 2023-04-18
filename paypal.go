package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/plutov/paypal/v4"
)

func NewPaypalClient() *paypal.Client {
	auth := paypal_auth
	if !auth.Initialized() {
		return nil
	}
	client, err := paypal.NewClient(auth.ClientID, auth.Secret, auth.APIBase)
	FatalIf(err)

	_, err = client.GetAccessToken(context.Background())
	FatalIf(err)
	return client
}

type eInvalidAmount struct {
	amount paypal.PurchaseUnitAmount
}

func (e eInvalidAmount) Error() string {
	return fmt.Sprintf("invalid purchase unit amount: %v", e.amount)
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

func transactionIncomingHandler(client *paypal.Client) http.Handler {
	return http.HandlerFunc(func(wr http.ResponseWriter, req *http.Request) {
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

		order, err := client.GetOrder(context.Background(), in.ProviderSession)
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
	})
}
