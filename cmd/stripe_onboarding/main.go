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
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cognitoidentityprovider"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
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

func unmarshalCreateCustomerEvents(event events.SQSEvent) ([]*createCustomerEvent, error) {
	events := []*createCustomerEvent{}
	for _, record := range event.Records {
		item := &createCustomerEvent{}
		err := json.Unmarshal([]byte(record.Body), item)
		if err != nil {
			return events, fmt.Errorf("unable to unmarshal event ID %s", record.MessageId)
		}
		item.SQSMessageID = record.MessageId
		item.SQSReceiptHandle = record.ReceiptHandle
		events = append(events, item)
	}
	return events, nil
}

func onboardCustomer(ctx context.Context, event events.SQSEvent) error {
	customerEvents, err := unmarshalCreateCustomerEvents(event)
	if err != nil {
		return err
	}
	requestCount := len(event.Records)
	wg := &sync.WaitGroup{}
	wg.Add(requestCount)
	chanStripe := make(chan resultStripe, requestCount)
	for _, customerEvent := range customerEvents {
		go createCustomers(wg, chanStripe, stripeClient.Customers, customerEvent)
	}
	wg.Wait()
	close(chanStripe)
	inputs, items, tableName, err := generatePutRequestInputBatches(requestCount, chanStripe)
	if err != nil {
		return err
	}
	requestCount = len(inputs) + len(items.Items)
	wg.Add(requestCount)
	chanDynamoDB := make(chan resultDB, requestCount)
	chanCognito := make(chan resultCognito, requestCount)
	for _, input := range inputs {
		go batchWriteItems(ctx, wg, chanDynamoDB, db, input, tableName)
	}
	for _, item := range items.Items {
		go writeStripeIDUserAttribute(ctx, wg, chanCognito, cognito, item)
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
	sqsBatchInputs, err := generateDeleteMessageInputBatches(requestCount, items)
	if err != nil {
		return err
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

func handler(ctx context.Context, event events.SQSEvent) error {
	log.Info(fmt.Sprintf("Handling %v events", len(event.Records)))
	err := onboardCustomer(ctx, event)
	if err != nil {
		return err
	}
	return nil
}

func main() {
	lambda.Start(handler)
}
