package main

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func Test_generatePutRequestInput(t *testing.T) {
	type args struct {
		item createCustomerEvent
	}
	tests := []struct {
		name    string
		args    args
		want    map[string]types.AttributeValue
		wantErr bool
	}{
		{
			name: "",
			args: args{
				item: createCustomerEvent{
					PK:               "",
					SK:               "",
					StripeCustomerID: "01234",
					CognitoUserID:    "56789",
					FirstName:        "first_example",
					SurName:          "sur_example",
					EmailAddress:     "example@example.com",
				},
			},
			want: map[string]types.AttributeValue{
				"PK":               &types.AttributeValueMemberS{Value: "USER#56789"},
				"SK":               &types.AttributeValueMemberS{Value: "USER#MAIDO"},
				"FirstName":        &types.AttributeValueMemberS{Value: "first_example"},
				"SurName":          &types.AttributeValueMemberS{Value: "sur_example"},
				"EmailAddress":     &types.AttributeValueMemberS{Value: "example@example.com"},
				"StripeCustomerID": &types.AttributeValueMemberS{Value: "01234"},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generatePutRequestInput(tt.args.item)
			if (err != nil) != tt.wantErr {
				t.Errorf("generatePutRequestInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generatePutRequestInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockBatchWriteItem struct {
	Response *dynamodb.BatchWriteItemOutput
}

func (m mockBatchWriteItem) BatchWriteItem(
	ctx context.Context,
	params *dynamodb.BatchWriteItemInput,
	optFns ...func(*dynamodb.Options),
) (*dynamodb.BatchWriteItemOutput, error) {
	return m.Response, nil
}

func Test_batchWriteItems(t *testing.T) {
	mockTableName := "mockTable"
	writeRequestInput := types.WriteRequest{
		PutRequest: &types.PutRequest{
			Item: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: "1234"},
				"SK": &types.AttributeValueMemberS{Value: "5678"},
			},
		},
	}
	writeRequestInput26Times := []types.WriteRequest{
		writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput,
		writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput,
		writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput,
		writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput,
		writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput, writeRequestInput,
		writeRequestInput,
	}
	wg := &sync.WaitGroup{}
	ctx := context.TODO()
	type args struct {
		ctx           context.Context
		mock          awsDynamoDBAPI
		wg            *sync.WaitGroup
		ch            chan resultDB
		input         *dynamodb.BatchWriteItemInput
		mockTableName string
	}

	tests := []struct {
		name string
		args args
	}{
		{
			name: "1_item",
			args: args{
				ctx: ctx,
				mock: mockBatchWriteItem{
					Response: &dynamodb.BatchWriteItemOutput{},
				},
				wg: wg,
				ch: make(chan resultDB),
				input: &dynamodb.BatchWriteItemInput{
					RequestItems: map[string][]types.WriteRequest{
						mockTableName: {writeRequestInput},
					},
				},
				mockTableName: mockTableName,
			},
		},
		{
			name: "2_items",
			args: args{
				ctx: ctx,
				mock: mockBatchWriteItem{
					Response: &dynamodb.BatchWriteItemOutput{},
				},
				wg: wg,
				ch: make(chan resultDB),
				input: &dynamodb.BatchWriteItemInput{
					RequestItems: map[string][]types.WriteRequest{
						mockTableName: {writeRequestInput, writeRequestInput},
					},
				},
				mockTableName: mockTableName,
			},
		},
		{
			// not working
			name: "26_items",
			args: args{
				ctx: ctx,
				mock: mockBatchWriteItem{
					Response: &dynamodb.BatchWriteItemOutput{},
				},
				// Response: &dynamodb.BatchWriteItemOutput{
				// 	UnprocessedItems: map[string][]types.WriteRequest{
				// 		mockTableName: {
				// 			writeRequestInput,
				// 		},
				// 	},
				// },
				wg: wg,
				ch: make(chan resultDB),
				input: &dynamodb.BatchWriteItemInput{
					RequestItems: map[string][]types.WriteRequest{
						mockTableName: writeRequestInput26Times,
					},
				},
				mockTableName: mockTableName,
			},
		},
	}
	wg.Add(len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchWriteItems(tt.args.ctx, tt.args.wg, tt.args.ch, tt.args.mock, tt.args.input, tt.args.mockTableName)
		})
		if len(tt.args.ch) > 0 {
			for x := range tt.args.ch {
				if x.Error != nil {
					t.Fatal(x.Error)
				}
			}
		}
	}
	wg.Wait()
}
