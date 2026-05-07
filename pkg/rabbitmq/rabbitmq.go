package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	ExchangeName = "loan_events"
	QueueName    = "loan_invested_queue"
	RoutingKey   = "loan.invested"
)

type RabbitMQ struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	mu      sync.Mutex
	url     string
}

func NewRabbitMQ(url string) (*RabbitMQ, error) {
	rmq := &RabbitMQ{url: url}
	if err := rmq.connect(); err != nil {
		return nil, err
	}
	return rmq, nil
}

func (r *RabbitMQ) connect() error {
	conn, err := amqp.Dial(r.url)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open RabbitMQ channel: %w", err)
	}

	// Declare exchange
	err = ch.ExchangeDeclare(
		ExchangeName,
		"direct",
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Declare queue
	q, err := ch.QueueDeclare(
		QueueName,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange
	err = ch.QueueBind(
		q.Name,
		RoutingKey,
		ExchangeName,
		false,
		nil,
	)
	if err != nil {
		ch.Close()
		conn.Close()
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	r.conn = conn
	r.channel = ch
	return nil
}

// Publish sends a message to the exchange. Returns error if RabbitMQ is unavailable.
func (r *RabbitMQ) Publish(ctx context.Context, routingKey string, body []byte) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.channel == nil || r.conn == nil || r.conn.IsClosed() {
		// Attempt reconnect
		if err := r.connect(); err != nil {
			return fmt.Errorf("rabbitmq unavailable: %w", err)
		}
	}

	err := r.channel.PublishWithContext(ctx,
		ExchangeName,
		routingKey,
		false, // mandatory
		false, // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

func (r *RabbitMQ) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.channel != nil {
		if err := r.channel.Close(); err != nil {
			log.Printf("[RabbitMQ] error closing channel: %v", err)
		}
	}
	if r.conn != nil {
		if err := r.conn.Close(); err != nil {
			log.Printf("[RabbitMQ] error closing connection: %v", err)
		}
	}
}
