package worker

import (
	"context"
	"log"
	"time"

	"github.com/hibatullaha/loan-engine/internal/repository"
	"github.com/hibatullaha/loan-engine/pkg/rabbitmq"
)

// OutboxRelay is a background worker that periodically polls the order_event table
// for PENDING events and retries publishing them to RabbitMQ.
// This is the "Relay" component of the Transactional Outbox Pattern.
type OutboxRelay struct {
	outboxRepo repository.OutboxRepositoryInterface
	rmq        rabbitmq.Publisher
	interval   time.Duration
	batchSize  int
}

func NewOutboxRelay(outboxRepo repository.OutboxRepositoryInterface, rmq rabbitmq.Publisher) *OutboxRelay {
	return &OutboxRelay{
		outboxRepo: outboxRepo,
		rmq:        rmq,
		interval:   1 * time.Minute, // poll every 1 minute
		batchSize:  50,
	}
}

// Start launches the relay in a goroutine. It runs until the context is cancelled.
func (r *OutboxRelay) Start(ctx context.Context) {
	go func() {
		log.Printf("[OutboxRelay] started, polling every %v", r.interval)
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				log.Println("[OutboxRelay] shutting down")
				return
			case <-ticker.C:
				r.processPendingEvents(ctx)
			}
		}
	}()
}

func (r *OutboxRelay) processPendingEvents(ctx context.Context) {
	events, err := r.outboxRepo.GetPendingEvents(ctx, r.batchSize)
	if err != nil {
		log.Printf("[OutboxRelay] ERROR: failed to fetch pending events: %v", err)
		return
	}

	if len(events) == 0 {
		return
	}

	log.Printf("[OutboxRelay] found %d pending events, processing...", len(events))

	for _, evt := range events {
		err := r.rmq.Publish(ctx, rabbitmq.RoutingKey, []byte(evt.Payload))
		if err != nil {
			log.Printf("[OutboxRelay] WARN: failed to publish event %d: %v", evt.ID, err)
			// Don't mark as failed on transient errors — leave as PENDING for next retry
			continue
		}

		// Published successfully — mark as SENT
		if err := r.outboxRepo.MarkAsSent(ctx, evt.ID); err != nil {
			log.Printf("[OutboxRelay] ERROR: published event %d but failed to mark as SENT: %v", evt.ID, err)
		} else {
			log.Printf("[OutboxRelay] INFO: event %d published and marked as SENT", evt.ID)
		}
	}
}
