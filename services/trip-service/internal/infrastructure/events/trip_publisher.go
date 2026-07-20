package events

import (
	"context"
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

func (p *TripPublisher) PublishTripCreated(ctx context.Context) error {
	return p.rabbitMQ.PublishMessage(ctx, "hello", "trip created")
}
