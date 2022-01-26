package main

import (
	"context"
	"os"
	"reflect"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

func Test_generateDeleteMessageInputBatches(t *testing.T) {
	const sqsQueueURL = "example_queue_url"
	deleteBatchEntry := types.DeleteMessageBatchRequestEntry{
		Id:            aws.String("12345"),
		ReceiptHandle: aws.String("67890"),
	}
	customerEvent := createCustomerEvent{
		SQSMessageID:     "12345",
		SQSReceiptHandle: "67890",
	}
	type args struct {
		requestCount int
		items        items
	}
	tests := []struct {
		name    string
		args    args
		want    []*sqs.DeleteMessageBatchInput
		wantErr bool
	}{
		{
			name: "1_item",
			args: args{
				requestCount: 1,
				items: items{
					Items: []createCustomerEvent{customerEvent},
				},
			},
			want: []*sqs.DeleteMessageBatchInput{{
				Entries:  []types.DeleteMessageBatchRequestEntry{deleteBatchEntry},
				QueueUrl: aws.String(sqsQueueURL),
			}},
			wantErr: false,
		},
		{
			name: "11_items",
			args: args{
				requestCount: 11,
				items: items{
					Items: []createCustomerEvent{
						customerEvent, customerEvent, customerEvent, customerEvent, customerEvent,
						customerEvent, customerEvent, customerEvent, customerEvent, customerEvent,
						customerEvent,
					},
				},
			},
			want: []*sqs.DeleteMessageBatchInput{
				{
					Entries: []types.DeleteMessageBatchRequestEntry{
						deleteBatchEntry, deleteBatchEntry, deleteBatchEntry, deleteBatchEntry, deleteBatchEntry,
						deleteBatchEntry, deleteBatchEntry, deleteBatchEntry, deleteBatchEntry, deleteBatchEntry,
					},
					QueueUrl: aws.String(sqsQueueURL),
				},
				{
					Entries:  []types.DeleteMessageBatchRequestEntry{deleteBatchEntry},
					QueueUrl: aws.String(sqsQueueURL),
				},
			},
			wantErr: false,
		},
	}
	os.Setenv("SQS_QUEUE_URL", sqsQueueURL)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := generateDeleteMessageInputBatches(tt.args.requestCount, tt.args.items)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateDeleteMessageInputBatches() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateDeleteMessageInputBatches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_generateDeleteMessageBatchRequestEntry(t *testing.T) {
	type args struct {
		SQSMessageID     string
		SQSReceiptHandle string
	}
	tests := []struct {
		name string
		args args
		want types.DeleteMessageBatchRequestEntry
	}{
		{
			name: "",
			args: args{
				SQSMessageID:     "12345",
				SQSReceiptHandle: "67890",
			},
			want: types.DeleteMessageBatchRequestEntry{
				Id:            aws.String("12345"),
				ReceiptHandle: aws.String("67890"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := generateDeleteMessageBatchRequestEntry(tt.args.SQSMessageID, tt.args.SQSReceiptHandle); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("generateDeleteMessageBatchRequestEntry() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_getFailedDeleteMessageIDS(t *testing.T) {
	type args struct {
		failed []types.BatchResultErrorEntry
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "1_entry",
			args: args{
				failed: []types.BatchResultErrorEntry{
					{
						Code:        aws.String("100"),
						Id:          aws.String("200"),
						SenderFault: false,
						Message:     aws.String("example"),
					},
				},
			},
			want: []string{"{\"id\":\"200\",\"sender_fault\":false,\"message\":\"example\"}"},
		}, {
			name: "2_entries",
			args: args{
				failed: []types.BatchResultErrorEntry{
					{
						Code:        aws.String("100"),
						Id:          aws.String("200"),
						SenderFault: false,
						Message:     aws.String("example"),
					},
					{
						Code:        aws.String("300"),
						Id:          aws.String("400"),
						SenderFault: false,
						Message:     aws.String("example_two"),
					},
				},
			},
			want: []string{
				"{\"id\":\"200\",\"sender_fault\":false,\"message\":\"example\"}",
				"{\"id\":\"400\",\"sender_fault\":false,\"message\":\"example_two\"}",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getFailedDeleteMessageIDS(tt.args.failed); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("getFailedDeleteMessageIDS() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockBatchDeleteMessage struct {
	Response *sqs.DeleteMessageBatchOutput
}

func (m mockBatchDeleteMessage) DeleteMessageBatch(
	ctx context.Context,
	params *sqs.DeleteMessageBatchInput,
	optFns ...func(*sqs.Options),
) (*sqs.DeleteMessageBatchOutput, error) {
	return m.Response, nil
}

func Test_batchDeleteMessages(t *testing.T) {
	ctx := context.TODO()
	wg := &sync.WaitGroup{}
	type args struct {
		ctx   context.Context
		wg    *sync.WaitGroup
		ch    chan resultSQS
		queue awsSQSAPI
		input *sqs.DeleteMessageBatchInput
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "1_entry",
			args: args{
				ctx: ctx,
				wg:  wg,
				ch:  make(chan resultSQS),
				queue: mockBatchDeleteMessage{
					Response: &sqs.DeleteMessageBatchOutput{},
				},
				input: &sqs.DeleteMessageBatchInput{
					Entries: []types.DeleteMessageBatchRequestEntry{
						{
							Id:            aws.String("12345"),
							ReceiptHandle: aws.String("67890"),
						},
					},
					QueueUrl: aws.String("example"),
				},
			},
		}, {
			name: "2_entries",
			args: args{
				ctx: ctx,
				wg:  wg,
				ch:  make(chan resultSQS),
				queue: mockBatchDeleteMessage{
					Response: &sqs.DeleteMessageBatchOutput{},
				},
				input: &sqs.DeleteMessageBatchInput{
					Entries: []types.DeleteMessageBatchRequestEntry{
						{
							Id:            aws.String("12345"),
							ReceiptHandle: aws.String("67890"),
						},
						{
							Id:            aws.String("abcde"),
							ReceiptHandle: aws.String("fghij"),
						},
					},
					QueueUrl: aws.String("example"),
				},
			},
		},
	}
	wg.Add(len(tests))
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batchDeleteMessages(tt.args.ctx, tt.args.wg, tt.args.ch, tt.args.queue, tt.args.input)
		})
	}
}
