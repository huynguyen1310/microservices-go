package domain

import (
	"ride-sharing/services/trip-service/pkg/types"
	pb "ride-sharing/shared/proto/trip"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RideFareModel struct {
	ID                bson.ObjectID          `bson:"_id,omitempty"`
	UserID            string                 `bson:"userID"`
	PackageSlug       string                 `bson:"packageSlug"` // ex: van, luxury, sedan
	TotalPriceInCents float64                `bson:"totalPriceInCents"`
	Route             *types.OsrmApiResponse `bson:"route"`
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
