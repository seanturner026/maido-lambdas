package main

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/stripe/stripe-go/v72"
)

type createCustomerEvent struct {
	PK               string `dynamodbav:"PK"`
	SK               string `dynamodbav:"SK"`
	StripeCustomerID string `dynamodbav:"StripeCustomerID"`
	SQSMessageID     string `dynamodbav:"-"`
	SQSReceiptHandle string `dynamodbav:"-"`
	CognitoUserID    string `dynamodbav:"-"                json:"cognitoUserID"`
	FirstName        string `dynamodbav:"FirstName"        json:"firstName"`
	SurName          string `dynamodbav:"SurName"          json:"surName"`
	EmailAddress     string `dynamodbav:"EmailAddress"     json:"email"`
}

type resultStripe struct {
	Message         string
	Event           createCustomerEvent
	PutRequestInput map[string]types.AttributeValue
	Error           error
}

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
