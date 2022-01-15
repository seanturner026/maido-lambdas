package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sync"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	log "github.com/sirupsen/logrus"
	"github.com/stripe/stripe-go/v72/client"
)

var (
	cognito      *cognitoidentityprovider.Client
	db           *dynamodb.Client
	queue        *sqs.Client
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
	queue = sqs.NewFromConfig(cfg)
}

type items struct {
	Items []createCustomerEvent
}

func onboardCustomer(wg *sync.WaitGroup, ch chan resultStripe, apiStripe stripeCustomerCreateAPI, event events.SQSMessage) {
	defer wg.Done()
	item := &createCustomerEvent{}
	err := json.Unmarshal([]byte(event.Body), item)
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to unmarshal event ID %s", event.MessageId), Error: err}
	}
	stripeCustomerID, err := createCustomer(apiStripe, item.EmailAddress, fmt.Sprintf("%s %s", item.FirstName, item.SurName))
	if err != nil {
		ch <- resultStripe{Message: fmt.Sprintf("Unable to create Customer for Cognito User ID %s", item.CognitoUserID), Error: err}
		return
	}
	item.StripeCustomerID = stripeCustomerID
	log.WithFields(log.Fields{"cognito_user_id": item.CognitoUserID, "stripe_customer_id": stripeCustomerID}).Info("Created stripe customer ID")
	putRequestInput, err := generatePutRequestInput(*item)
	if err != nil {
		ch <- resultStripe{
			Message: fmt.Sprintf(
				"Unable to generate DynamoDB input for cognitoUserID %s stripeCustomerID %s",
				item.CognitoUserID,
				stripeCustomerID,
			),
			Error: err,
		}
		return
	}
	item.SQSMessageID = event.MessageId
	item.SQSReceiptHandle = event.ReceiptHandle
	ch <- resultStripe{PutRequestInput: putRequestInput, Event: *item}
}

func handler(ctx context.Context, event events.SQSEvent) error {
	log.Info(fmt.Sprintf("Handling %v events", len(event.Records)))
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
	// TODO: Test 25 items
	for res := range chanStripe {
		if res.Error != nil {
			log.WithFields(log.Fields{"error": res.Error}).Error(res.Message)
			break
		}
		items.Items = append(items.Items, res.Event)
		putItemRequest := types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: res.PutRequestInput,
			},
		}
		input.RequestItems[tableName] = append(input.RequestItems[tableName], putItemRequest)
		if i%24 == 0 || i == requestCount {
			inputs = append(inputs, input)
			input = &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					tableName: {},
				},
			}
		}
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
			log.WithFields(log.Fields{"cognito_user_ids": ch.UserIDS, "error": ch.Error}).Error(ch.Message)
		}
	}
	for ch := range chanCognito {
		if ch.Error != nil {
			log.WithFields(log.Fields{"cognito_user_id": ch.UserID, "error": ch.Error}).Error(ch.Message)
		}
	}
	requestCount = int(math.Ceil(float64(len(items.Items)) / 10))
	queueURL, ok := os.LookupEnv("SQS_QUEUE_URL")
	if !ok {
		log.Error("Environment variable SQS_QUEUE_URL is not set")
		return nil
	}
	sqsBatchInputs := []*sqs.DeleteMessageBatchInput{}
	sqsBatchInput := &sqs.DeleteMessageBatchInput{
		QueueUrl: aws.String(queueURL),
	}
	for i, item := range items.Items {
		itemInputEntry := generateDeleteMessageBatchRequestEntry(item.SQSMessageID, item.SQSReceiptHandle)
		sqsBatchInput.Entries = append(sqsBatchInput.Entries, itemInputEntry)
		if i%9 == 0 || i == requestCount {
			sqsBatchInputs = append(sqsBatchInputs, sqsBatchInput)
			sqsBatchInput = &sqs.DeleteMessageBatchInput{
				QueueUrl: aws.String(queueURL),
			}
		}
	}
	wg.Add(requestCount)
	chanSQS := make(chan resultSQS, requestCount)
	for _, batch := range sqsBatchInputs {
		go batchDeleteMessages(ctx, wg, chanSQS, queue, batch)
	}
	wg.Wait()
	close(chanSQS)
	for ch := range chanSQS {
		if ch.Error != nil {
			log.WithFields(log.Fields{"failed_delete_messages": ch.FailedDeleteMessages, "error": ch.Error}).Error(ch.Message)
		}
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
