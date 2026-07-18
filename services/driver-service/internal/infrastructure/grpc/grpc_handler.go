package grpc

import (
	"context"
	"ride-sharing/services/driver-service/internal/service"

	pb "ride-sharing/shared/proto/driver"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type grpcHandler struct {
	pb.UnimplementedDriverServiceServer
	svc *service.Service
}

func NewGrpcHandler(s *grpc.Server, svc *service.Service) *grpcHandler {
	h := &grpcHandler{svc: svc}
	pb.RegisterDriverServiceServer(s, h)
	return h
}

func (h *grpcHandler) RegisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method RegisterDriver is not implemented")
}

func (h *grpcHandler) UnRegisterDriver(ctx context.Context, req *pb.RegisterDriverRequest) (*pb.RegisterDriverResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method UnRegisterDriver is not implemented")
}
