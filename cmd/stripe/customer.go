package main

import "github.com/stripe/stripe-go/v72"

type stripeCustomerCreateAPI interface {
	New(params *stripe.CustomerParams) (*stripe.Customer, error)
}

func createCustomer(api stripeCustomerCreateAPI, customerEmail, customerName string) (string, error) {
	params := &stripe.CustomerParams{
		Email: stripe.String(customerEmail),
		InvoiceSettings: &stripe.CustomerInvoiceSettingsParams{
			CustomFields:         []*stripe.CustomerInvoiceCustomFieldParams{},
			DefaultPaymentMethod: new(string),
		},
		Name:      stripe.String(customerName),
		Source:    &stripe.SourceParams{},
		Tax:       &stripe.CustomerTaxParams{},
		TaxExempt: stripe.String("none"),
	}
	customer, err := api.New(params)
	if err != nil {
		return "", err
	}
	customerID := customer.ID
	return customerID, nil
}
