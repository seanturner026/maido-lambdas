package main

import (
	"reflect"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func Test_unmarshalCreateCustomerEvents(t *testing.T) {
	type args struct {
		event events.SQSEvent
	}
	tests := []struct {
		name    string
		args    args
		want    []*createCustomerEvent
		wantErr bool
	}{
		{
			name: "",
			args: args{
				event: events.SQSEvent{
					Records: []events.SQSMessage{{
						MessageId:     "123456789",
						ReceiptHandle: "23456789",
						Body:          "{\"cognitoUserID\": \"56789\", \"email\": \"example@example.com\", \"firstName\": \"first_example\", \"surName\": \"sur_example\"}",
					}},
				},
			},
			want: []*createCustomerEvent{{
				PK:               "",
				SK:               "",
				StripeCustomerID: "",
				SQSMessageID:     "123456789",
				SQSReceiptHandle: "23456789",
				CognitoUserID:    "56789",
				FirstName:        "first_example",
				SurName:          "sur_example",
				EmailAddress:     "example@example.com",
			}},
			wantErr: false,
		},
		{
			name: "",
			args: args{
				event: events.SQSEvent{
					Records: []events.SQSMessage{{
						MessageId:     "123456789",
						ReceiptHandle: "23456789",
						Body:          "",
					}},
				},
			},
			want:    []*createCustomerEvent{},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unmarshalCreateCustomerEvents(tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("unmarshalCreateCustomerEvents() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("unmarshalCreateCustomerEvents() = %v, want %v", got, tt.want)
			}
		})
	}
}
