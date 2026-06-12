package queue

import (
	"context"
	"fmt"
)

func (r *RabbitMQ) PublishReleaseRequested(ctx context.Context, message ReleaseMessage) error {
	if message.Event == "" {
		message.Event = EventReleaseRequested
	}
	if err := publishContext(ctx, r.ch, r.cfg.Exchange, r.cfg.RoutingKey, message); err != nil {
		return fmt.Errorf("publish release requested: %w", err)
	}
	return nil
}
