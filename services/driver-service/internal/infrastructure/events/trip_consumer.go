package events

import (
	"context"
	"encoding/json"
	"log"
	"ride-sharing/shared/contracts"
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
	return tc.rabbitMQ.ConsumeMessages(messaging.FindAvailableDriversQueue, func(ctx context.Context, msg amqp.Delivery) error {
		var tripEvent contracts.AmqpMessage
		if err := json.Unmarshal(msg.Body, &tripEvent); err != nil {
			return err
		}

		var payload messaging.TripEvent
		if err := json.Unmarshal(tripEvent.Data, &payload); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			return err
		}

		tripJSON, err := json.Marshal(payload.Trip)
		if err != nil {
			log.Printf("failed to marshal trip: %v", err)
			return err
		}
		log.Printf("driver received trip event for user: %s, trip: %s", tripEvent.OwnerID, string(tripJSON))
		return nil
	})
}
