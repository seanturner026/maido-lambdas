package main

import (
	"testing"

	"github.com/stripe/stripe-go/v72"
)

type mockStripeCustomer struct {
	Response *stripe.Customer
}

func (m mockStripeCustomer) New(params *stripe.CustomerParams) (*stripe.Customer, error) {
	return m.Response, nil
}

func Test_createCustomer(t *testing.T) {
	type args struct {
		mock          mockStripeCustomer
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
				mock: mockStripeCustomer{
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
			got, err := createCustomer(tt.args.mock, tt.args.customerEmail, tt.args.customerName)
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
