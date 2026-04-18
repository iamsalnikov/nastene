package rabbit

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/ThreeDotsLabs/watermill"
	"github.com/ThreeDotsLabs/watermill-amqp/v2/pkg/amqp"
	"github.com/ThreeDotsLabs/watermill/message"
)

const topicDelimiter = "::"

func amqpConfig(amqpURI, consumerGroup string) amqp.Config {
	return amqp.Config{
		Connection: amqp.ConnectionConfig{
			AmqpURI: amqpURI,
		},

		Marshaler: amqp.DefaultMarshaler{},

		Exchange: amqp.ExchangeConfig{
			GenerateName: func(topic string) string {
				return "application"
			},
			Type:    "topic",
			Durable: true,
		},
		Queue: amqp.QueueConfig{
			GenerateName: func(topic string) string {
				return fmt.Sprintf("%s.%s", consumerGroup, topic)
			},
			Durable: true,
		},
		QueueBind: amqp.QueueBindConfig{
			GenerateRoutingKey: func(topic string) string {
				topic = strings.TrimPrefix(topic, consumerGroup+".")

				parts := strings.SplitN(topic, topicDelimiter, 2)

				return parts[0]
			},
		},
		Publish: amqp.PublishConfig{
			GenerateRoutingKey: func(topic string) string {
				return topic
			},
		},
		Consume: amqp.ConsumeConfig{
			Qos: amqp.QosConfig{
				PrefetchCount: 1,
			},
		},
		TopologyBuilder: &amqp.DefaultTopologyBuilder{},
	}
}

func NewPublisher(amqpURI, consumerGroup string) (message.Publisher, error) {
	pub, err := amqp.NewPublisher(
		amqpConfig(amqpURI, consumerGroup),
		watermill.NewSlogLogger(slog.Default()),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create rabbit publisher: %w", err)
	}

	return pub, nil
}

func NewPublisherFromEnv(dsnEnv, groupEnv string) (message.Publisher, error) {
	return NewPublisher(os.Getenv(dsnEnv), os.Getenv(groupEnv))
}

func NewPublisherFromEnvOrPanic(dsnEnv, groupEnv string) message.Publisher {
	pub, err := NewPublisherFromEnv(dsnEnv, groupEnv)
	if err != nil {
		panic(err)
	}

	return pub
}

func NewSubscriber(amqpURI, consumerGroup string) (message.Subscriber, error) {
	sub, err := amqp.NewSubscriber(
		amqpConfig(amqpURI, consumerGroup),
		watermill.NewSlogLogger(slog.Default()),
	)
	if err != nil {
		return nil, fmt.Errorf("unable to create rabbit subscriber: %w", err)
	}

	return sub, nil
}

func NewSubscriberFromEnv(dsnEnv, groupEnv string) (message.Subscriber, error) {
	return NewSubscriber(os.Getenv(dsnEnv), os.Getenv(groupEnv))
}

func NewSubscriberFromEnvOrPanic(dsnEnv, groupEnv string) message.Subscriber {
	sub, err := NewSubscriberFromEnv(dsnEnv, groupEnv)
	if err != nil {
		panic(err)
	}

	return sub
}
