package q

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill/message"
)

type publisher interface {
	Publish(topic string, messages ...*message.Message) error
}

type subscriber interface {
	Subscribe(ctx context.Context, topic string) (<-chan *message.Message, error)
}

func Publish(pub publisher, topic string, data any) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("unable to marshal data: %w", err)
	}

	msg := message.NewMessage(watermill.NewUUID(), payload)

	if err := pub.Publish(topic, msg); err != nil {
		return fmt.Errorf("unable to publish message: %w", err)
	}

	return nil
}

func Subscribe[T any](
	ctx context.Context,
	sub subscriber,
	topic string,
	logger *slog.Logger,
	handler func(context.Context, T) error,
) error {
	if logger == nil {
		logger = slog.Default()
	}

	messages, err := sub.Subscribe(ctx, topic)
	if err != nil {
		return fmt.Errorf("unable to subscribe to topic: %w", err)
	}

	for msg := range messages {
		var data T

		log := logger.With("message_id", msg.UUID, "topic", topic)

		if err := json.Unmarshal(msg.Payload, &data); err != nil {
			log.With("error", err).Error("unable to unmarshal message")
			msg.Ack()

			continue
		}

		if err := handler(msg.Context(), data); err != nil {
			log.With("error", err).Error("unable to handle message")
			msg.Nack()

			continue
		}

		msg.Ack()
	}

	return nil
}
