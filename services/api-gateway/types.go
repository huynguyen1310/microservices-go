package main

import "ride-sharing/shared/types"

type previewTripRequest struct {
	UserID      string           `json:"user_id"`
	Pickup      types.Coordinate `json:"pickup"`
	Destination types.Coordinate `json:"destination"`
}
