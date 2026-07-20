package grpc

import (
	"context"
	"log"
	"ride-sharing/services/trip-service/internal/domain"
	"ride-sharing/services/trip-service/internal/infrastructure/events"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type gRPCHandler struct {
	pb.UnimplementedTripServiceServer
	service   domain.TripService
	publisher *events.TripPublisher
}

func NewGRPCHandler(server *grpc.Server, service domain.TripService, publisher *events.TripPublisher) *gRPCHandler {
	handler := &gRPCHandler{
		service:   service,
		publisher: publisher,
	}
	pb.RegisterTripServiceServer(server, handler)
	return handler
}

func (h *gRPCHandler) CreateTrip(ctx context.Context, req *pb.CreateTripRequest) (*pb.CreateTripResponse, error) {
	rideFare, err := h.service.GetAndValidateFare(ctx, req.GetRideFareID(), req.GetUserID(), nil)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get and validate fare: %v", err)
	}

	trip, err := h.service.CreateTrip(ctx, rideFare)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to create trip: %v", err)
	}

	if err := h.publisher.PublishTripCreated(ctx, trip); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to publish trip created: %v", err)
	}

	return &pb.CreateTripResponse{
		TripID: trip.ID.Hex(),
	}, nil
}

func (h *gRPCHandler) PreviewTrip(ctx context.Context, req *pb.PreviewTripRequest) (*pb.PreviewTripResponse, error) {
	pickup := req.GetStartLocation()
	destination := req.GetEndLocation()

	route, err := h.service.GetRoute(ctx,
		&types.Coordinate{
			Latitude:  pickup.Latitude,
			Longitude: pickup.Longitude,
		},
		&types.Coordinate{
			Latitude:  destination.Latitude,
			Longitude: destination.Longitude,
		})

	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "internal server error: %v", err)
	}

	estimatedFares := h.service.EstimatePackagePriceWithRoute(route)
	fares, err := h.service.GenerateTripFare(ctx, estimatedFares, req.GetUserID(), route)
	if err != nil {
		log.Println(err)
		return nil, status.Errorf(codes.Internal, "Failed to generate trip fare: %v", err)
	}

	return &pb.PreviewTripResponse{
		Route:     route.ToProto(),
		RideFares: domain.ToRideFaresProto(fares),
	}, nil
}
