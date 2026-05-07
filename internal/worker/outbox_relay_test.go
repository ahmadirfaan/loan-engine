package worker

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/hibatullaha/loan-engine/internal/domain"
	"github.com/hibatullaha/loan-engine/internal/mocks"
	"github.com/stretchr/testify/assert"
)

func TestOutboxRelay_ProcessPendingEvents_Success(t *testing.T) {
	outboxRepo := &mocks.MockOutboxRepository{}
	publisher := &mocks.MockPublisher{}

	events := []*domain.OrderEvent{
		{ID: 1, Payload: `{"loan_id":1}`, Status: domain.OutboxStatusPending, EventType: "LOAN_INVESTED"},
		{ID: 2, Payload: `{"loan_id":2}`, Status: domain.OutboxStatusPending, EventType: "LOAN_INVESTED"},
	}

	outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
		return events, nil
	}

	publishedIDs := []int64{}
	publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return nil
	}
	outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error {
		publishedIDs = append(publishedIDs, id)
		return nil
	}

	relay := NewOutboxRelay(outboxRepo, publisher)
	relay.processPendingEvents(context.Background())

	assert.Equal(t, []int64{1, 2}, publishedIDs)
}

func TestOutboxRelay_ProcessPendingEvents_NoEvents(t *testing.T) {
	outboxRepo := &mocks.MockOutboxRepository{}
	publisher := &mocks.MockPublisher{}

	outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
		return nil, nil
	}

	relay := NewOutboxRelay(outboxRepo, publisher)
	// Should not panic
	relay.processPendingEvents(context.Background())
}

func TestOutboxRelay_ProcessPendingEvents_FetchError(t *testing.T) {
	outboxRepo := &mocks.MockOutboxRepository{}
	publisher := &mocks.MockPublisher{}

	outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
		return nil, fmt.Errorf("db connection lost")
	}

	relay := NewOutboxRelay(outboxRepo, publisher)
	// Should not panic — just logs
	relay.processPendingEvents(context.Background())
}

func TestOutboxRelay_ProcessPendingEvents_PublishFails(t *testing.T) {
	outboxRepo := &mocks.MockOutboxRepository{}
	publisher := &mocks.MockPublisher{}

	events := []*domain.OrderEvent{
		{ID: 1, Payload: `{"loan_id":1}`, Status: domain.OutboxStatusPending, EventType: "LOAN_INVESTED"},
	}

	outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
		return events, nil
	}

	publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return fmt.Errorf("rabbitmq connection refused")
	}

	markSentCalled := false
	outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error {
		markSentCalled = true
		return nil
	}

	relay := NewOutboxRelay(outboxRepo, publisher)
	relay.processPendingEvents(context.Background())

	// MarkAsSent should NOT be called since publish failed
	assert.False(t, markSentCalled)
}

func TestOutboxRelay_ProcessPendingEvents_MarkSentFails(t *testing.T) {
	outboxRepo := &mocks.MockOutboxRepository{}
	publisher := &mocks.MockPublisher{}

	events := []*domain.OrderEvent{
		{ID: 1, Payload: `{"loan_id":1}`, Status: domain.OutboxStatusPending, EventType: "LOAN_INVESTED"},
	}

	outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
		return events, nil
	}
	publisher.PublishFn = func(ctx context.Context, routingKey string, body []byte) error {
		return nil
	}
	outboxRepo.MarkAsSentFn = func(ctx context.Context, id int64) error {
		return fmt.Errorf("update failed")
	}

	relay := NewOutboxRelay(outboxRepo, publisher)
	// Should not panic — just logs
	relay.processPendingEvents(context.Background())
}

func TestOutboxRelay_Start_ContextCancellation(t *testing.T) {
	outboxRepo := &mocks.MockOutboxRepository{}
	publisher := &mocks.MockPublisher{}

	outboxRepo.GetPendingEventsFn = func(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
		return nil, nil
	}

	relay := &OutboxRelay{
		outboxRepo: outboxRepo,
		rmq:        publisher,
		interval:   50 * time.Millisecond, // fast tick for test
		batchSize:  10,
	}

	ctx, cancel := context.WithCancel(context.Background())
	relay.Start(ctx)

	// Give it time to tick at least once
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Give goroutine time to exit
	time.Sleep(100 * time.Millisecond)
}
