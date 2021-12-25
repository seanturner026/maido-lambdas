package main

import (
	"context"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
)

type mockAdminUpdateUserAttributes struct {
	Response *cognitoidentityprovider.AdminUpdateUserAttributesOutput
}

func (m mockAdminUpdateUserAttributes) AdminUpdateUserAttributes(
	ctx context.Context,
	params *cognitoidentityprovider.AdminUpdateUserAttributesInput,
	optFns ...func(*cognitoidentityprovider.Options),
) (*cognitoidentityprovider.AdminUpdateUserAttributesOutput, error) {
	return m.Response, nil
}

func Test_writeStripeIDUserAttribute(t *testing.T) {
	wg := &sync.WaitGroup{}
	ctx := context.TODO()
	type args struct {
		ctx        context.Context
		wg         *sync.WaitGroup
		ch         chan resultCognito
		mock       awsCognitoIdentityProviderAPI
		userPoolID string
		event      createCustomerEvent
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "",
			args: args{
				ctx: ctx,
				wg:  wg,
				ch:  make(chan resultCognito),
				mock: &mockAdminUpdateUserAttributes{
					Response: &cognitoidentityprovider.AdminUpdateUserAttributesOutput{},
				},
				userPoolID: "example_user_pool_id",
				event: createCustomerEvent{
					PK:               "",
					SK:               "",
					StripeCustomerID: "01234",
					CognitoUserID:    "56789",
					FirstName:        "first_example",
					SurName:          "sur_example",
					EmailAddress:     "example@example.com",
				},
			},
		},
	}
	wg.Add(len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writeStripeIDUserAttribute(tt.args.ctx, tt.args.wg, tt.args.ch, tt.args.mock, tt.args.userPoolID, tt.args.event)
		})
	}
	wg.Wait()
}
