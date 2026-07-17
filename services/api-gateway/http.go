package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"ride-sharing/services/api-gateway/grpc_clients"
	"ride-sharing/shared/contracts"
	pb "ride-sharing/shared/proto/trip"

	"google.golang.org/grpc/status"
)

type tripStartRequest struct {
	RideFareID string `json:"rideFareID"`
	UserID     string `json:"userID"`
}

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	var reqBody previewTripRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	defer r.Body.Close()

	if reqBody.UserID == "" {
		writeJSON(w, http.StatusBadRequest, "userID is required")
		return
	}

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, err.Error())
		return
	}

	defer tripService.Close()

	tripPreview, err := tripService.Client.PreviewTrip(r.Context(), reqBody.toProto())
	if err != nil {
		log.Printf("Failed to preview a trip: %v", err)
		http.Error(w, "Failed to preview trip", http.StatusInternalServerError)
		return
	}

	response := contracts.APIResponse{Data: tripPreview}
	writeJSON(w, http.StatusCreated, response)
}

func handleTripStart(w http.ResponseWriter, r *http.Request) {
	var reqBody tripStartRequest
	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		writeJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	defer r.Body.Close()

	if reqBody.UserID == "" {
		writeJSON(w, http.StatusBadRequest, "userID is required")
		return
	}
	if reqBody.RideFareID == "" {
		writeJSON(w, http.StatusBadRequest, "rideFareID is required")
		return
	}

	tripService, err := grpc_clients.NewTripServiceClient()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer tripService.Close()

	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	resp, err := tripService.Client.CreateTrip(ctx, &pb.CreateTripRequest{
		RideFareID: reqBody.RideFareID,
		UserID:     reqBody.UserID,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			writeJSON(w, int(st.Code()), st.Message())
		} else {
			log.Printf("Failed to create trip: %v", err)
			writeJSON(w, http.StatusInternalServerError, "failed to create trip")
		}
		return
	}

	writeJSON(w, http.StatusOK, contracts.APIResponse{Data: resp})
}
