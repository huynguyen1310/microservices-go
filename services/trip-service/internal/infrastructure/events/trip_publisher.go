package events

import (
	"context"
	"encoding/json"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/messaging"
)

type TripPublisher struct {
	rabbitMQ *messaging.RabbitMQ
}

func NewTripPublisher(rabbitMQ *messaging.RabbitMQ) *TripPublisher {
	return &TripPublisher{
		rabbitMQ: rabbitMQ,
	}
}

func (p *TripPublisher) PublishTripCreated(ctx context.Context, trip *domain.TripModel) error {

	payload := messaging.TripEvent{
		Trip: trip.ToProto(),
	}

	tripEventJson, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	return p.rabbitMQ.PublishMessage(ctx, contracts.TripEventCreated, contracts.AmqpMessage{
		OwnerID: trip.UserID,
		Data:    tripEventJson,
	})
}
