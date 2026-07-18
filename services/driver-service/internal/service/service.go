package service

import (
	pb "ride-sharing/shared/proto/driver"
)

type Service struct {
	drivers []*driverInMap
}

type driverInMap struct {
	Driver *pb.Driver
}

func NewService() *Service {
	return &Service{
		drivers: make([]*driverInMap, 0),
	}
}
