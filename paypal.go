package main

import (
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
