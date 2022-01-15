package main

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
)

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
