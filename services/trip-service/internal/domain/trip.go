package domain

import (
	"context"
	"ride-sharing/shared/types"

	tripType "ride-sharing/services/trip-service/pkg/types"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TripModel struct {
	ID       bson.ObjectID
	UserID   string
	Status   string
	RideFare *RideFareModel
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*tripType.OsrmApiResponse, error)
}
