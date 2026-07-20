package messaging

import (
	pb "ride-sharing/shared/proto/trip"
)

const (
	FindAvailableDriversQueue = "find_available_drivers"
)

type TripEvent struct {
	Trip *pb.Trip `json:"trip"`
}
