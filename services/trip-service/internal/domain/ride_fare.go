package domain

import (
	pb "ride-sharing/shared/proto/trip"

	"go.mongodb.org/mongo-driver/v2/bson"
)

type RideFareModel struct {
	ID                bson.ObjectID
	UserID            string
	PackageSlug       string
	TotalPriceInCents float64
}

func (r *RideFareModel) ToProto() *pb.RideFares {
	return &pb.RideFares{
		Id:                r.ID.Hex(),
		UserID:            r.UserID,
		PackageSlug:       r.PackageSlug,
		TotalPriceInCents: r.TotalPriceInCents,
	}
}

func ToRideFaresProto(fares []*RideFareModel) []*pb.RideFares {
	var protoFares []*pb.RideFares
	for _, fare := range fares {
		protoFares = append(protoFares, fare.ToProto())
	}
	return protoFares
}
