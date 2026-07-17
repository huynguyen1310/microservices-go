package domain

import (
	"context"
	"ride-sharing/shared/types"

	tripType "ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type TripModel struct {
	ID       bson.ObjectID
	UserID   string
	Status   string
	RideFare *RideFareModel
	Driver   *pb.TripDriver
}

type TripRepository interface {
	CreateTrip(ctx context.Context, trip *TripModel) (*TripModel, error)
	SaveRideFare(ctx context.Context, fare *RideFareModel) error
	GetRideFareById(ctx context.Context, id string) (*RideFareModel, error)
}

type TripService interface {
	CreateTrip(ctx context.Context, fare *RideFareModel) (*TripModel, error)
	GetRoute(ctx context.Context, pickup, destination *types.Coordinate) (*tripType.OsrmApiResponse, error)
	EstimatePackagePriceWithRoute(route *tripType.OsrmApiResponse) []*RideFareModel
	GenerateTripFare(ctx context.Context, fare []*RideFareModel, userID string, route *tripType.OsrmApiResponse) ([]*RideFareModel, error)
	GetAndValidateFare(ctx context.Context, fareID, userID string, route *tripType.OsrmApiResponse) (*RideFareModel, error)
}
