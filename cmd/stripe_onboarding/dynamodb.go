package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
)

func generatePutRequestInputBatches(requestCount int, chanStripe chan resultStripe) ([]*dynamodb.BatchWriteItemInput, items, string, error) {
	tableName, ok := os.LookupEnv("DYNAMODB_TABLE_NAME")
	if !ok {
		return []*dynamodb.BatchWriteItemInput{}, items{}, "", fmt.Errorf("environment variable DYNAMODB_TABLE_NAME is not set")
	}
	inputs := []*dynamodb.BatchWriteItemInput{}
	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: {},
		},
	}
	writeRequest := []types.WriteRequest{}
	i := 1
	items := &items{}
	for res := range chanStripe {
		if res.Error != nil {
			log.WithFields(log.Fields{"error": res.Error}).Error(res.Message)
			continue // TODO handle better
		}
		items.Items = append(items.Items, res.Event)
		putItemRequest := &types.PutRequest{
			Item: res.PutRequestInput,
		}
		writeRequest = append(writeRequest, types.WriteRequest{PutRequest: putItemRequest})
		input.RequestItems[tableName] = writeRequest
		if i%25 == 0 || i == requestCount {
			inputs = append(inputs, input)
			input = &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					tableName: {},
				},
			}
			writeRequest = []types.WriteRequest{}
		}
		i++
	}
	return inputs, *items, tableName, nil
}

func generatePutRequestInput(item createCustomerEvent) (map[string]types.AttributeValue, error) {
	item.PK = fmt.Sprintf("USER#%s", item.CognitoUserID)
	item.SK = "USER#MAIDO"
	putItemInput, err := attributevalue.MarshalMap(item)
	if err != nil {
		return map[string]types.AttributeValue{}, err
	}
	return putItemInput, err
}

func extractCognitoUserIDSFromBatchWriteInput(input dynamodb.BatchWriteItemInput, tableName string) ([]string, error) {
	type cognitoUser struct {
		ID string `dynamodbav:"PK"`
	}
	cognitoUserIDS := []string{}
	for _, item := range input.RequestItems[tableName] {
		id := &cognitoUser{}
		err := attributevalue.UnmarshalMap(item.PutRequest.Item, id)
		if err != nil {
			return cognitoUserIDS, err
		}
		cognitoUserIDS = append(cognitoUserIDS, strings.Split(id.ID, "USER#")[1])
	}
	return cognitoUserIDS, nil
}

type resultDB struct {
	Message string
	UserIDS []string
	Error   error
}

type awsDynamoDBAPI interface {
	BatchWriteItem(
		ctx context.Context,
		params *dynamodb.BatchWriteItemInput,
		optFns ...func(*dynamodb.Options),
	) (*dynamodb.BatchWriteItemOutput, error)
}

func batchWriteItems(
	ctx context.Context,
	wg *sync.WaitGroup,
	ch chan resultDB,
	db awsDynamoDBAPI,
	input *dynamodb.BatchWriteItemInput,
	tableName string,
) {
	defer wg.Done()
	resp, err := db.BatchWriteItem(ctx, input)
	if err != nil {
		cognitoUserIDs, err := extractCognitoUserIDSFromBatchWriteInput(*input, tableName)
		if err != nil {
			log.Error("Error batch writing and extracting cognito user IDs from input object")
		}
		ch <- resultDB{Error: err, UserIDS: cognitoUserIDs, Message: "Error writing Stripe Customer IDs to dynamodb"}
		return
	}

	_, ok := resp.UnprocessedItems[tableName]
	if ok {
		for {
			input = &dynamodb.BatchWriteItemInput{
				RequestItems: resp.UnprocessedItems,
			}
			resp, err = db.BatchWriteItem(context.TODO(), input)
			if err != nil {
				cognitoUserIDs, err := extractCognitoUserIDSFromBatchWriteInput(*input, tableName)
				if err != nil {
					log.Error("Error batch writing and extracting cognito user IDs from input object")
				}
				ch <- resultDB{Error: err, UserIDS: cognitoUserIDs, Message: "Error writing Stripe Customer IDs to dynamodb"}
				return
			}
			if resp.UnprocessedItems == nil {
				break
			}
		}
	}
}
