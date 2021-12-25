package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v72/client"
)

var (
	cognito      *cognitoidentityprovider.Client
	db           *dynamodb.Client
	stripeClient *client.API
)

func init() {
	log.SetFormatter(&log.JSONFormatter{})
	stripeAPIKey := os.Getenv("STRIPE_API_KEY")
	stripeClient = client.New(stripeAPIKey, nil)
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("unable to load SDK config, %v", err)
	}
	db = dynamodb.NewFromConfig(cfg)
	cognito = cognitoidentityprovider.NewFromConfig(cfg)
}

type items struct {
	Items []createCustomerEvent
}

type createCustomerEvent struct {
	PK               string `dynamodbav:"PK"`
	SK               string `dynamodbav:"SK"`
	StripeCustomerID string `dynamodbav:"StripeCustomerID"`
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

type resultDB struct {
	Error error
}

type resultCognito struct {
	Error error
}

func onboardCustomer(wg *sync.WaitGroup, ch chan resultStripe, apiStripe stripeCustomerCreateAPI, event events.SQSMessage) {
	defer wg.Done()
	item := &createCustomerEvent{}
	err := json.Unmarshal([]byte(event.Body), item)
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to create unmarshall event %s", event.MessageId), Error: err}
	}
	stripeCustomerID, err := createCustomer(apiStripe, item.EmailAddress, fmt.Sprintf("%s %s", item.FirstName, item.SurName))
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to create Customer for Cognito User ID %s", item.CognitoUserID), Error: err}
		return
	}
	item.StripeCustomerID = stripeCustomerID
	putRequestInput, err := generatePutRequestInput(*item)
	if err != nil {
		ch <- resultStripe{
			Message: fmt.Sprintf(
				"Unable to generate DynamoDB input -- item %s cognitoUserID %s stripeCustomerID %s",
				event.MessageId,
				item.CognitoUserID,
				stripeCustomerID,
			),
			Error: err,
		}
		return
	}
	ch <- resultStripe{PutRequestInput: putRequestInput, Event: *item}
}

func handler(event events.SQSEvent) error {
	wg := &sync.WaitGroup{}
	requestCount := len(event.Records)
	wg.Add(requestCount)
	chanStripe := make(chan resultStripe, requestCount)
	for _, record := range event.Records {
		go onboardCustomer(wg, chanStripe, stripeClient.Customers, record)
	}
	wg.Wait()
	close(chanStripe)
	tableName, ok := os.LookupEnv("DYNAMODB_TABLE_NAME")
	if !ok {
		log.Error("Environment variable DYNAMODB_TABLE_NAME is not set")
		return nil
	}
	inputs := []*dynamodb.BatchWriteItemInput{}
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: {},
		},
	}
	i := 0
	items := &items{}
	for res := range chanStripe {
		if res.Error != nil {
			log.WithFields(log.Fields{"error": res.Error}).Error(res.Message)
			break
		}
		items.Items = append(items.Items, res.Event)
		if (i+1)%25 == 0 || i+1 == requestCount {
			inputs = append(inputs, input)
			if (i+1)%25 == 0 {
				input = &dynamodb.BatchWriteItemInput{
					RequestItems: map[string][]types.WriteRequest{
						tableName: {},
					},
				}
			}
		}
		putItemRequest := types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: res.PutRequestInput,
			},
		}
		input.RequestItems[tableName] = append(input.RequestItems[tableName], putItemRequest)
		i++
	}
	requestCount = len(inputs) + len(items.Items)
	wg.Add(requestCount)
	chanDynamoDB := make(chan resultDB, requestCount)
	chanCognito := make(chan resultCognito, requestCount)
	userPoolID, ok := os.LookupEnv("USER_POOL_ID")
	if !ok {
		log.Error("Environment variable USER_POOL_ID is not set")
		return nil
	}
	ctx := context.TODO()
	for _, input := range inputs {
		go batchWriteItems(ctx, wg, chanDynamoDB, db, input, tableName)
	}
	for _, item := range items.Items {
		go writeStripeIDUserAttribute(ctx, wg, chanCognito, cognito, userPoolID, item)
	}
	wg.Wait()
	close(chanDynamoDB)
	close(chanCognito)
	for ch := range chanDynamoDB {
		if ch.Error != nil {
			log.Error(ch.Error)
		}
	}
	for ch := range chanCognito {
		if ch.Error != nil {
			log.Error(ch.Error)
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
