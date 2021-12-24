package main

// import "github.com/stripe/stripe-go/v72"

// type stripePaymentIntentAPI interface {
// 	New(params *stripe.PaymentIntentParams) (*stripe.PaymentIntent, error)
// }

// func createCustomerPaymentIntent(api stripePaymentIntentAPI, customerID string) error {
// 	params := &stripe.PaymentIntentParams{
// 		Params:                    stripe.Params{},
// 		Amount:                    stripe.Int64(10000),
// 		AutomaticPaymentMethods:   &stripe.PaymentIntentAutomaticPaymentMethodsParams{Enabled: stripe.Bool(true)},
// 		CaptureMethod:             new(string),
// 		ClientSecret:              new(string),
// 		Confirm:                   new(bool),
// 		ConfirmationMethod:        new(string),
// 		Currency:                  stripe.String(string(stripe.CurrencyUSD)),
// 		Customer:                  stripe.String(customerID),
// 		Description:               new(string),
// 		Mandate:                   new(string),
// 		MandateData:               &stripe.PaymentIntentMandateDataParams{},
// 		OnBehalfOf:                new(string),
// 		PaymentMethod:             new(string),
// 		PaymentMethodData:         &stripe.PaymentIntentPaymentMethodDataParams{},
// 		PaymentMethodOptions:      &stripe.PaymentIntentPaymentMethodOptionsParams{},
// 		PaymentMethodTypes:        []*string{stripe.String("card"), stripe.String("wallet")},
// 		ReceiptEmail:              new(string),
// 		ReturnURL:                 new(string),
// 		SetupFutureUsage:          stripe.String("off_session"),
// 		Shipping:                  &stripe.ShippingDetailsParams{},
// 		StatementDescriptor:       new(string),
// 		StatementDescriptorSuffix: new(string),
// 		TransferData:              &stripe.PaymentIntentTransferDataParams{},
// 		TransferGroup:             new(string),
// 		ErrorOnRequiresAction:     new(bool),
// 		OffSession:                new(bool),
// 		UseStripeSDK:              new(bool),
// 	}
// 	pi, _ := paymentintent.New(params)
// 	return nil
// }
