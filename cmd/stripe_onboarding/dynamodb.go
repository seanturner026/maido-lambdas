package main

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	log "github.com/sirupsen/logrus"
)

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
