package rabbitmq

import "context"

// Publisher defines the interface for publishing messages.
// This allows mocking in unit tests.
type Publisher interface {
	Publish(ctx context.Context, routingKey string, body []byte) error
}
