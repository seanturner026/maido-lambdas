package main

import (
	"sync"
	"testing"

	"github.com/stripe/stripe-go/v72"
)

type mockStripeCustomer struct {
	Response *stripe.Customer
}

func (m mockStripeCustomer) New(params *stripe.CustomerParams) (*stripe.Customer, error) {
	return m.Response, nil
}

func Test_createCustomers(t *testing.T) {
	wg := &sync.WaitGroup{}
	type args struct {
		wg        *sync.WaitGroup
		ch        chan resultStripe
		apiStripe stripeCustomerCreateAPI
		event     *createCustomerEvent
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "",
			args: args{
				wg: wg,
				ch: make(chan resultStripe),
				apiStripe: mockStripeCustomer{
					Response: &stripe.Customer{
						ID: "01234567890",
					},
				},
				event: &createCustomerEvent{
					CognitoUserID: "01234567890",
					FirstName:     "first",
					SurName:       "last",
					EmailAddress:  "example@example.com",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			go createCustomers(tt.args.wg, tt.args.ch, tt.args.apiStripe, tt.args.event)
		})
	}
	wg.Wait()
}

func Test_createCustomer(t *testing.T) {
	type args struct {
		api           mockStripeCustomer
		customerEmail string
		customerName  string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "",
			args: args{
				api: mockStripeCustomer{
					Response: &stripe.Customer{
						ID: "01234567890",
					},
				},
				customerEmail: "foo.bar@gmail.com",
				customerName:  "Boo Far",
			},
			want:    "01234567890",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := createCustomer(tt.args.api, tt.args.customerEmail, tt.args.customerName)
			if (err != nil) != tt.wantErr {
				t.Errorf("createCustomer() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("createCustomer() = %v, want %v", got, tt.want)
			}
		})
	}
}
