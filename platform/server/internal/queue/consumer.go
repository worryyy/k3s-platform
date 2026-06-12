package queue

import (
	"context"
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Handler func(context.Context, ReleaseMessage) error

func (r *RabbitMQ) Consume(ctx context.Context, handler Handler) error {
	if err := r.ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set consumer qos: %w", err)
	}
	deliveries, err := r.ch.Consume(r.cfg.Queue, r.cfg.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start release consumer: %w", err)
	}

	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("release deliveries channel closed")
			}
			r.handleDelivery(ctx, delivery, handler)
		}
	}
}

func (r *RabbitMQ) handleDelivery(ctx context.Context, delivery amqp.Delivery, handler Handler) {
	message, err := decodeMessage(delivery.Body)
	if err != nil {
		r.logger.Error("invalid release message", "error", err)
		_ = delivery.Nack(false, false)
		return
	}
	if err := handler(ctx, message); err != nil {
		r.logger.Error("release message handler failed", "release_id", message.ReleaseID, "error", err)
		_ = delivery.Nack(false, true)
		return
	}
	_ = delivery.Ack(false)
}
