package main

import (
	"context"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func Test_generatePutRequestInputBatches(t *testing.T) {
	const tableName = "example_table_name"
	customerEvent := &createCustomerEvent{
		PK:               "USER#56789",
		SK:               "USER#MAIDO",
		StripeCustomerID: "01234",
		SQSMessageID:     "123456789",
		SQSReceiptHandle: "23456789",
		CognitoUserID:    "56789",
		FirstName:        "first_example",
		SurName:          "sur_example",
		EmailAddress:     "example@example.com",
	}
	type args struct {
		requestCount int
		chanStripe   chan resultStripe
	}
	putRequestInput := map[string]types.AttributeValue{
		"PK":               &types.AttributeValueMemberS{Value: "USER#56789"},
		"SK":               &types.AttributeValueMemberS{Value: "USER#MAIDO"},
		"FirstName":        &types.AttributeValueMemberS{Value: "first_example"},
		"SurName":          &types.AttributeValueMemberS{Value: "sur_example"},
		"EmailAddress":     &types.AttributeValueMemberS{Value: "example@example.com"},
		"StripeCustomerID": &types.AttributeValueMemberS{Value: "01234"},
	}
	chanInput := &resultStripe{
		Message:         "example",
		Event:           *customerEvent,
		PutRequestInput: putRequestInput,
		Error:           nil,
	}
	zeroItemsChanStripe := make(chan resultStripe)
	oneItemsChanStripe := make(chan resultStripe, 1)
	oneItemsChanStripe <- *chanInput
	twoItemsChanStripe := make(chan resultStripe, 2)
	twoItemsChanStripe <- *chanInput
	twoItemsChanStripe <- *chanInput
	twentySixItemsChanStripe := make(chan resultStripe, 26)
	for i := 0; i < 26; i++ {
		twentySixItemsChanStripe <- *chanInput
	}
	close(zeroItemsChanStripe)
	close(oneItemsChanStripe)
	close(twoItemsChanStripe)
	close(twentySixItemsChanStripe)
	writeRequest := types.WriteRequest{
		PutRequest: &types.PutRequest{
			Item: putRequestInput,
		},
	}
	tests := []struct {
		name    string
		args    args
		want    []*dynamodb.BatchWriteItemInput
		want1   items
		want2   string
		wantErr bool
	}{
		{
			name: "0_items",
			args: args{
				requestCount: 0,
				chanStripe:   zeroItemsChanStripe,
			},
			want:    []*dynamodb.BatchWriteItemInput{},
			want1:   items{},
			want2:   tableName,
			wantErr: false,
		},
		{
			name: "1_item",
			args: args{
				requestCount: 1,
				chanStripe:   oneItemsChanStripe,
			},
			want: []*dynamodb.BatchWriteItemInput{{
				RequestItems: map[string][]types.WriteRequest{
					tableName: {writeRequest},
				},
			}},
			want1: items{
				Items: []createCustomerEvent{*customerEvent},
			},
			want2:   tableName,
			wantErr: false,
		},
		{
			name: "2_items",
			args: args{
				requestCount: 2,
				chanStripe:   twoItemsChanStripe,
			},
			want: []*dynamodb.BatchWriteItemInput{{
				RequestItems: map[string][]types.WriteRequest{
					tableName: {writeRequest, writeRequest},
				},
			}},
			want1: items{
				Items: []createCustomerEvent{*customerEvent, *customerEvent},
			},
			want2:   tableName,
			wantErr: false,
		},
		{
			name: "26_items",
			args: args{
				requestCount: 26,
				chanStripe:   twentySixItemsChanStripe,
			},
			want: []*dynamodb.BatchWriteItemInput{{
				RequestItems: map[string][]types.WriteRequest{
					tableName: {
						writeRequest, writeRequest, writeRequest, writeRequest, writeRequest,
						writeRequest, writeRequest, writeRequest, writeRequest, writeRequest,
						writeRequest, writeRequest, writeRequest, writeRequest, writeRequest,
						writeRequest, writeRequest, writeRequest, writeRequest, writeRequest,
						writeRequest, writeRequest, writeRequest, writeRequest, writeRequest,
					},
				},
			},
				{
					RequestItems: map[string][]types.WriteRequest{
						tableName: {writeRequest},
					},
				},
			},
			want1: items{
				Items: []createCustomerEvent{
					*customerEvent, *customerEvent, *customerEvent, *customerEvent, *customerEvent,
					*customerEvent, *customerEvent, *customerEvent, *customerEvent, *customerEvent,
					*customerEvent, *customerEvent, *customerEvent, *customerEvent, *customerEvent,
					*customerEvent, *customerEvent, *customerEvent, *customerEvent, *customerEvent,
					*customerEvent, *customerEvent, *customerEvent, *customerEvent, *customerEvent,
					*customerEvent,
				},
			},
			want2:   tableName,
			wantErr: false,
		},
	}
	err := os.Setenv("DYNAMODB_TABLE_NAME", tableName)
	if err != nil {
		t.Fatal("error setting DYNAMODB_TABLE_NAME environment variable")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2, err := generatePutRequestInputBatches(tt.args.requestCount, tt.args.chanStripe)
			if (err != nil) != tt.wantErr {
				t.Errorf("generatePutRequestInputBatches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generatePutRequestInputBatches() got = %v, want %v", got, tt.want)
			}
			if !reflect.DeepEqual(got1, tt.want1) {
				t.Errorf("generatePutRequestInputBatches() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("generatePutRequestInputBatches() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

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

func Test_extractCognitoUserIDSFromBatchWriteInput(t *testing.T) {
	type args struct {
		input     dynamodb.BatchWriteItemInput
		tableName string
	}
	tests := []struct {
		name    string
		args    args
		want    []string
		wantErr bool
	}{
		{
			name: "",
			args: args{
				input: dynamodb.BatchWriteItemInput{
					RequestItems: map[string][]types.WriteRequest{
						"mockTable": {
							{
								PutRequest: &types.PutRequest{
									Item: map[string]types.AttributeValue{
										"PK": &types.AttributeValueMemberS{Value: "USER#12345"},
									},
								},
							}, {
								PutRequest: &types.PutRequest{
									Item: map[string]types.AttributeValue{
										"PK": &types.AttributeValueMemberS{Value: "USER#67890"},
									},
								},
							},
						},
					},
				},
				tableName: "mockTable",
			},
			want:    []string{"12345", "67890"},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractCognitoUserIDSFromBatchWriteInput(tt.args.input, tt.args.tableName)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractCognitoUserIDSFromBatchWriteInput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractCognitoUserIDSFromBatchWriteInput() = %v, want %v", got, tt.want)
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
		db            awsDynamoDBAPI
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
				db: mockBatchWriteItem{
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
		}, {
			name: "2_items",
			args: args{
				ctx: ctx,
				db: mockBatchWriteItem{
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
		}, {
			// not working
			name: "26_items",
			args: args{
				ctx: ctx,
				db: mockBatchWriteItem{
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
			batchWriteItems(tt.args.ctx, tt.args.wg, tt.args.ch, tt.args.db, tt.args.input, tt.args.mockTableName)
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
