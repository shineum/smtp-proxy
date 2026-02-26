package queue

import (
	"context"
	"fmt"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
)

// sqsAPI abstracts the AWS SQS client for testability.
type sqsAPI interface {
	SendMessage(ctx context.Context, input *sqsSendInput) (*sqsSendOutput, error)
	ReceiveMessage(ctx context.Context, input *sqsReceiveInput) (*sqsReceiveOutput, error)
	DeleteMessage(ctx context.Context, input *sqsDeleteInput) error
	ChangeMessageVisibility(ctx context.Context, input *sqsChangeVisibilityInput) error
}

// sqsSendInput mirrors the fields needed for SQS SendMessage.
type sqsSendInput struct {
	QueueURL     string
	MessageBody  string
	DelaySeconds int32
}

// sqsSendOutput contains the result of a successful SendMessage call.
type sqsSendOutput struct {
	MessageID string
}

// sqsReceiveInput mirrors the fields needed for SQS ReceiveMessage.
type sqsReceiveInput struct {
	QueueURL            string
	MaxNumberOfMessages int32
	WaitTimeSeconds     int32
	VisibilityTimeout   int32
}

// sqsReceiveOutput contains the messages returned by ReceiveMessage.
type sqsReceiveOutput struct {
	Messages []sqsReceivedMessage
}

// sqsReceivedMessage represents a single message received from SQS.
type sqsReceivedMessage struct {
	MessageID     string
	ReceiptHandle string
	Body          string
}

// sqsDeleteInput mirrors the fields needed for SQS DeleteMessage.
type sqsDeleteInput struct {
	QueueURL      string
	ReceiptHandle string
}

// sqsChangeVisibilityInput mirrors the fields needed for SQS ChangeMessageVisibility.
type sqsChangeVisibilityInput struct {
	QueueURL          string
	ReceiptHandle     string
	VisibilityTimeout int32
}

// awsSQSClient wraps the real AWS SQS SDK client and implements sqsAPI.
type awsSQSClient struct {
	client *sqs.Client
}

// newAWSSQSClient creates an awsSQSClient configured for the given region.
func newAWSSQSClient(region string) (*awsSQSClient, error) {
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}
	return &awsSQSClient{client: sqs.NewFromConfig(cfg)}, nil
}

// SendMessage sends a message to the specified SQS queue.
func (c *awsSQSClient) SendMessage(ctx context.Context, input *sqsSendInput) (*sqsSendOutput, error) {
	out, err := c.client.SendMessage(ctx, &sqs.SendMessageInput{
		QueueUrl:     &input.QueueURL,
		MessageBody:  &input.MessageBody,
		DelaySeconds: input.DelaySeconds,
	})
	if err != nil {
		return nil, err
	}
	var msgID string
	if out.MessageId != nil {
		msgID = *out.MessageId
	}
	return &sqsSendOutput{MessageID: msgID}, nil
}

// ReceiveMessage long-polls the specified SQS queue for messages.
func (c *awsSQSClient) ReceiveMessage(ctx context.Context, input *sqsReceiveInput) (*sqsReceiveOutput, error) {
	out, err := c.client.ReceiveMessage(ctx, &sqs.ReceiveMessageInput{
		QueueUrl:            &input.QueueURL,
		MaxNumberOfMessages: input.MaxNumberOfMessages,
		WaitTimeSeconds:     input.WaitTimeSeconds,
		VisibilityTimeout:   input.VisibilityTimeout,
	})
	if err != nil {
		return nil, err
	}

	messages := make([]sqsReceivedMessage, 0, len(out.Messages))
	for _, m := range out.Messages {
		messages = append(messages, sqsReceivedMessage{
			MessageID:     derefString(m.MessageId),
			ReceiptHandle: derefString(m.ReceiptHandle),
			Body:          derefString(m.Body),
		})
	}
	return &sqsReceiveOutput{Messages: messages}, nil
}

// DeleteMessage deletes a message from the specified SQS queue.
func (c *awsSQSClient) DeleteMessage(ctx context.Context, input *sqsDeleteInput) error {
	_, err := c.client.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      &input.QueueURL,
		ReceiptHandle: &input.ReceiptHandle,
	})
	return err
}

// ChangeMessageVisibility changes the visibility timeout of a message.
func (c *awsSQSClient) ChangeMessageVisibility(ctx context.Context, input *sqsChangeVisibilityInput) error {
	_, err := c.client.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          &input.QueueURL,
		ReceiptHandle:     &input.ReceiptHandle,
		VisibilityTimeout: input.VisibilityTimeout,
	})
	return err
}

// derefString safely dereferences a string pointer, returning "" for nil.
func derefString(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
