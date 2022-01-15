package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider/types"
)

type resultCognito struct {
	Message string
	UserID  string
	Error   error
}

type awsCognitoIdentityProviderAPI interface {
	AdminUpdateUserAttributes(
		ctx context.Context,
		params *cognitoidentityprovider.AdminUpdateUserAttributesInput,
		optFns ...func(*cognitoidentityprovider.Options),
	) (*cognitoidentityprovider.AdminUpdateUserAttributesOutput, error)
}

func writeStripeIDUserAttribute(
	ctx context.Context,
	wg *sync.WaitGroup,
	ch chan resultCognito,
	cognito awsCognitoIdentityProviderAPI,
	event createCustomerEvent,
) {
	defer wg.Done()
	userPoolID, ok := os.LookupEnv("USER_POOL_ID")
	if !ok {
		ch <- resultCognito{Error: fmt.Errorf("environment variable USER_POOL_ID is not set"), UserID: event.CognitoUserID, Message: "Unable to add user attribute"}
	}
	input := &cognitoidentityprovider.AdminUpdateUserAttributesInput{
		UserAttributes: []types.AttributeType{{
			Name:  aws.String("custom:stripe_customer_id"),
			Value: aws.String(event.StripeCustomerID),
		}},
		UserPoolId: aws.String(userPoolID),
		Username:   aws.String(event.CognitoUserID),
	}
	_, err := cognito.AdminUpdateUserAttributes(ctx, input)
	if err != nil {
		ch <- resultCognito{Error: err, UserID: event.CognitoUserID, Message: "Unable to add user attribute"}
	}
}
