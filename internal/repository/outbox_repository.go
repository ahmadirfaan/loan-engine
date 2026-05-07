package repository

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/hibatullaha/loan-engine/internal/domain"
)

type OutboxRepository struct {
	db *sql.DB
}

func NewOutboxRepository(db *sql.DB) *OutboxRepository {
	return &OutboxRepository{db: db}
}

// GetPendingEvents fetches all events with PENDING status (for the outbox relay).
func (r *OutboxRepository) GetPendingEvents(ctx context.Context, limit int) ([]*domain.OrderEvent, error) {
	query := `
		SELECT id, payload, status, event_type, created_at, updated_at
		FROM order_event
		WHERE status = $1
		ORDER BY created_at ASC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, domain.OutboxStatusPending, limit)
	if err != nil {
		return nil, fmt.Errorf("outbox get pending: %w", err)
	}
	defer rows.Close()

	var events []*domain.OrderEvent
	for rows.Next() {
		evt := &domain.OrderEvent{}
		err := rows.Scan(&evt.ID, &evt.Payload, &evt.Status, &evt.EventType, &evt.CreatedAt, &evt.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("outbox scan: %w", err)
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}

// MarkAsSent updates outbox event status to SENT.
func (r *OutboxRepository) MarkAsSent(ctx context.Context, id int64) error {
	query := `UPDATE order_event SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, domain.OutboxStatusSent, id)
	if err != nil {
		return fmt.Errorf("outbox mark sent: %w", err)
	}
	return nil
}

// MarkAsFailed updates outbox event status to FAILED.
func (r *OutboxRepository) MarkAsFailed(ctx context.Context, id int64) error {
	query := `UPDATE order_event SET status = $1, updated_at = NOW() WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, domain.OutboxStatusFailed, id)
	if err != nil {
		return fmt.Errorf("outbox mark failed: %w", err)
	}
	return nil
}
