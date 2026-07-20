package events

import (
	"context"
	"log"
	"ride-sharing/shared/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TripConsumer struct {
	rabbitMQ *messaging.RabbitMQ
}

func NewTripConsumer(rabbitMQ *messaging.RabbitMQ) *TripConsumer {
	return &TripConsumer{
		rabbitMQ: rabbitMQ,
	}
}

func (tc *TripConsumer) Listen() error {
	return tc.rabbitMQ.ConsumeMessages("hello", func(ctx context.Context, msg amqp.Delivery) error {
		log.Printf("driver received message: %s", string(msg.Body))
		return nil
	})
}
