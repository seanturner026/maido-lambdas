package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
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

type awsDynamoDBAPI interface {
	BatchWriteItem(ctx context.Context, params *dynamodb.BatchWriteItemInput, optFns ...func(*dynamodb.Options)) (*dynamodb.BatchWriteItemOutput, error)
}

func batchWriteItems(ctx context.Context, db awsDynamoDBAPI, wg *sync.WaitGroup, ch chan resultDB, input *dynamodb.BatchWriteItemInput) {
	defer wg.Done()
	_, err := db.BatchWriteItem(ctx, input)
	if err != nil {
		ch <- resultDB{Error: err}
	}
}
