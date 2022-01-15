package main

import (
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
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

func createCustomers(wg *sync.WaitGroup, ch chan resultStripe, apiStripe stripeCustomerCreateAPI, event *createCustomerEvent) {
	defer wg.Done()
	stripeCustomerID, err := createCustomer(apiStripe, event.EmailAddress, fmt.Sprintf("%s %s", event.FirstName, event.SurName))
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to create Customer for Cognito User ID %s", event.CognitoUserID), Error: err}
		return
	}
	event.StripeCustomerID = stripeCustomerID
	log.WithFields(log.Fields{"cognito_user_id": event.CognitoUserID, "stripe_customer_id": stripeCustomerID}).Info("Created stripe customer ID")
	putRequestInput, err := generatePutRequestInput(*event)
	if err != nil {
		ch <- resultStripe{
			Message: fmt.Sprintf(
				"Unable to generate DynamoDB input for cognitoUserID %s stripeCustomerID %s",
				event.CognitoUserID,
				stripeCustomerID,
			),
			Error: err,
		}
		return
	}
	ch <- resultStripe{PutRequestInput: putRequestInput, Event: *event}
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
