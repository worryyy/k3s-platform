package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Config struct {
	URL         string
	Exchange    string
	Queue       string
	DLQ         string
	RoutingKey  string
	ConsumerTag string
}

type RabbitMQ struct {
	conn   *amqp.Connection
	ch     *amqp.Channel
	cfg    Config
	logger *slog.Logger
}

func NewRabbitMQ(cfg Config, logger *slog.Logger) (*RabbitMQ, error) {
	if cfg.URL == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	if cfg.Exchange == "" {
		cfg.Exchange = "platform.release.exchange"
	}
	if cfg.Queue == "" {
		cfg.Queue = "platform.release.requested.queue"
	}
	if cfg.DLQ == "" {
		cfg.DLQ = "platform.release.dlq"
	}
	if cfg.RoutingKey == "" {
		cfg.RoutingKey = "release.requested"
	}
	if cfg.ConsumerTag == "" {
		cfg.ConsumerTag = "platform-release-worker"
	}
	if logger == nil {
		logger = slog.Default()
	}

	conn, err := amqp.Dial(cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("dial rabbitmq: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("open rabbitmq channel: %w", err)
	}

	client := &RabbitMQ{conn: conn, ch: ch, cfg: cfg, logger: logger}
	if err := client.declare(); err != nil {
		_ = client.Close()
		return nil, err
	}
	return client, nil
}

func (r *RabbitMQ) Close() error {
	if r == nil {
		return nil
	}
	if r.ch != nil {
		_ = r.ch.Close()
	}
	if r.conn != nil {
		return r.conn.Close()
	}
	return nil
}

func (r *RabbitMQ) declare() error {
	if err := r.ch.ExchangeDeclare(r.cfg.Exchange, amqp.ExchangeDirect, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare release exchange: %w", err)
	}
	if _, err := r.ch.QueueDeclare(r.cfg.DLQ, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare release dlq: %w", err)
	}
	args := amqp.Table{
		"x-dead-letter-exchange":    "",
		"x-dead-letter-routing-key": r.cfg.DLQ,
	}
	if _, err := r.ch.QueueDeclare(r.cfg.Queue, true, false, false, false, args); err != nil {
		return fmt.Errorf("declare release queue: %w", err)
	}
	if err := r.ch.QueueBind(r.cfg.Queue, r.cfg.RoutingKey, r.cfg.Exchange, false, nil); err != nil {
		return fmt.Errorf("bind release queue: %w", err)
	}
	return nil
}

func encodeMessage(message ReleaseMessage) ([]byte, error) {
	body, err := json.Marshal(message)
	if err != nil {
		return nil, fmt.Errorf("encode release message: %w", err)
	}
	return body, nil
}

func decodeMessage(body []byte) (ReleaseMessage, error) {
	var message ReleaseMessage
	if err := json.Unmarshal(body, &message); err != nil {
		return ReleaseMessage{}, fmt.Errorf("decode release message: %w", err)
	}
	if message.ReleaseID == "" || message.Service == "" || message.Environment == "" || message.Event == "" {
		return ReleaseMessage{}, fmt.Errorf("release message missing required fields")
	}
	return message, nil
}

func publishContext(ctx context.Context, ch *amqp.Channel, exchange, routingKey string, message ReleaseMessage) error {
	body, err := encodeMessage(message)
	if err != nil {
		return err
	}
	return ch.PublishWithContext(ctx, exchange, routingKey, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Body:         body,
	})
}
