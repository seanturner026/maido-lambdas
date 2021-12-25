package main

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	"github.com/aws/aws-sdk-go-v2/service/sqs/types"
	log "github.com/sirupsen/logrus"
)

func generateDeleteMessageBatchRequestEntry(SQSMessageID, SQSReceiptHandle string) types.DeleteMessageBatchRequestEntry {
	return types.DeleteMessageBatchRequestEntry{
		Id:            aws.String(SQSMessageID),
		ReceiptHandle: aws.String(SQSReceiptHandle),
	}
}

func getFailedDeleteMessageIDS(failed []types.BatchResultErrorEntry) []string {
	type failureResultSQS struct {
		ID          string `json:"id"`
		SenderFault bool   `json:"sender_fault"`
		Message     string `json:"message"`
	}
	failures := []string{}
	for _, failure := range failed {
		failureJSON, err := json.Marshal(failureResultSQS{ID: *failure.Id, SenderFault: failure.SenderFault, Message: *failure.Message})
		if err != nil {
			log.WithFields(log.Fields{"id": *failure.Id, "sender_fault": failure.SenderFault, "message": failure.Message}).
				Error("Unable to marshal failureResultSQS")
			continue
		}
		failures = append(failures, string(failureJSON))
	}
	return failures
}

type resultSQS struct {
	Message              string
	FailedDeleteMessages []string
	Error                error
}

type awsSQSAPI interface {
	DeleteMessageBatch(ctx context.Context, params *sqs.DeleteMessageBatchInput, optFns ...func(*sqs.Options)) (*sqs.DeleteMessageBatchOutput, error)
}

func batchDeleteMessages(
	ctx context.Context,
	wg *sync.WaitGroup,
	ch chan resultSQS,
	queue awsSQSAPI,
	input *sqs.DeleteMessageBatchInput,
) {
	defer wg.Done()
	resp, err := queue.DeleteMessageBatch(ctx, input)
	if err != nil {
		ch <- resultSQS{Error: err, Message: "Unable to delete message batch"}
	}
	if len(resp.Failed) > 0 {
		outstandingMessages := getFailedDeleteMessageIDS(resp.Failed)
		ch <- resultSQS{Error: err, FailedDeleteMessages: outstandingMessages, Message: "Messages failed to batch delete"}
	}
}
