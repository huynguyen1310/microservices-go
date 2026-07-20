package events

import (
	"context"
	"encoding/json"
	"log"
	"ride-sharing/services/driver-service/internal/service"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"

	amqp "github.com/rabbitmq/amqp091-go"
)

type TripConsumer struct {
	rabbitMQ *messaging.RabbitMQ
	service  *service.Service
}

func NewTripConsumer(rabbitMQ *messaging.RabbitMQ, service *service.Service) *TripConsumer {
	return &TripConsumer{
		rabbitMQ: rabbitMQ,
		service:  service,
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

		switch msg.RoutingKey {
		case contracts.TripEventCreated, contracts.TripEventDriverNotInterested:
			return tc.handleFindAndNotifyDrivers(ctx, payload)
		}

		log.Printf("unknown trip event: %+v", payload)

		return nil
	})
}

func (tc *TripConsumer) handleFindAndNotifyDrivers(ctx context.Context, payload messaging.TripEvent) error {
	suitableIDs := tc.service.FindAvailableDrivers(payload.Trip.SelectedFare.PackageSlug)

	log.Printf("Found suitable drivers %v", len(suitableIDs))

	if len(suitableIDs) == 0 {
		// Notify the driver that no drivers are available
		if err := tc.rabbitMQ.PublishMessage(ctx, contracts.TripEventNoDriversFound, contracts.AmqpMessage{
			OwnerID: payload.Trip.UserID,
		}); err != nil {
			log.Printf("Failed to publish message to exchange: %v", err)
			return err
		}

		return nil
	}

	suitableDriverID := suitableIDs[0]

	marshalledEvent, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	// Notify the driver about a potential trip
	if err := tc.rabbitMQ.PublishMessage(ctx, contracts.DriverCmdTripRequest, contracts.AmqpMessage{
		OwnerID: suitableDriverID,
		Data:    marshalledEvent,
	}); err != nil {
		log.Printf("Failed to publish message to exchange: %v", err)
		return err
	}

	return nil
}
