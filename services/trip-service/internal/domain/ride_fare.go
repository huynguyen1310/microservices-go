package domain

import (
	pb "ride-sharing/shared/proto/trip"

	tripTypes "ride-sharing/services/trip-service/pkg/types"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RideFareModel struct {
	ID                bson.ObjectID
	UserID            string
	PackageSlug       string
	TotalPriceInCents float64
	Route             *tripTypes.OsrmApiResponse
}

func (r *RideFareModel) ToProto() *pb.RideFare {
	return &pb.RideFare{
		Id:                r.ID.Hex(),
		UserID:            r.UserID,
		PackageSlug:       r.PackageSlug,
		TotalPriceInCents: r.TotalPriceInCents,
	}
}

func ToRideFaresProto(fares []*RideFareModel) []*pb.RideFare {
	var protoFares []*pb.RideFare
	for _, fare := range fares {
		protoFares = append(protoFares, fare.ToProto())
	}
	return protoFares
}
