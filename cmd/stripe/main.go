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
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v72/client"
)

var (
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
}

type createCustomerEvent struct {
	PK               string `dynamodbav:"PK"`
	SK               string `dynamodbav:"SK"`
	StripeCustomerID string `dynamodbav:"StripeCustomerID"`
	CognitoUserID    string `json:"cognitoUserID" dynamodbav:"-"`
	FirstName        string `json:"firstName      dynamodbav:FirstName"`
	SurName          string `json:"surName        dynamodbav:SurName"`
	EmailAddress     string `json:"emailAddress   dynamodbav:EmailAddress"`
}

type resultStripe struct {
	Message         string
	PutRequestInput map[string]types.AttributeValue
	Error           error
}

type resultDB struct {
	Error error
}

func onboardCustomer(wg *sync.WaitGroup, ch chan resultStripe, event events.SQSMessage, apiStripe stripeCustomerCreateAPI, apiDynamoDB awsDynamoDBAPI) {
	defer wg.Done()
	item := &createCustomerEvent{}
	err := json.Unmarshal([]byte(event.Body), item)
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to unmarshal -- item %s", event.MessageId), Error: err}
		return
	}
	stripeCustomerID, err := createCustomer(apiStripe, item.EmailAddress, fmt.Sprintf("%s %s", item.FirstName, item.SurName))
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to create Customer -- item %s", event.MessageId), Error: err}
		return
	}
	item.StripeCustomerID = stripeCustomerID
	putRequestInput, err := generatePutRequestInput(*item)
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to update Dynamo -- item %s cognitoUserID %s stripeCustomerID %s", event.MessageId, item.CognitoUserID, stripeCustomerID), Error: err}
		return
	}
	ch <- resultStripe{PutRequestInput: putRequestInput}
}

func handler(sqsEvent events.SQSEvent) error {
	wg := &sync.WaitGroup{}
	requestCount := len(sqsEvent.Records)
	wg.Add(requestCount)
	chanStripe := make(chan resultStripe, requestCount)
	for _, message := range sqsEvent.Records {
		go onboardCustomer(wg, chanStripe, message, stripeClient.Customers, db)
	}
	wg.Wait()
	tableName := os.Getenv("DYNAMODB_TABLE_NAME")
	inputs := []*dynamodb.BatchWriteItemInput{}
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: {},
		},
	}
	i := 0
	for res := range chanStripe {
		if res.Error != nil {
			log.WithFields(log.Fields{"error": res.Error}).Error(res.Message)
			break
		}
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
	requestCount = len(inputs)
	wg.Add(requestCount)
	ctx := context.TODO()
	chanDynamodb := make(chan resultDB, requestCount)
	for _, input := range inputs {
		go batchWriteItems(ctx, db, wg, chanDynamodb, input)
	}
	wg.Wait()
	for ch := range chanDynamodb {
		if ch.Error != nil {
			log.Error(ch.Error)
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
