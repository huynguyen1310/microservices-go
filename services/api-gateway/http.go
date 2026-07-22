package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"

	"ride-sharing/services/api-gateway/grpc_clients"
	"ride-sharing/shared/contracts"
	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"
	pb "ride-sharing/shared/proto/trip"
	"ride-sharing/shared/tracing"

	"github.com/stripe/stripe-go/v81"
	"github.com/stripe/stripe-go/v81/webhook"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/status"
)

var getTracer = func() trace.Tracer { return tracing.GetTracer("api-gateway") }

type tripStartRequest struct {
	RideFareID string `json:"rideFareID"`
	UserID     string `json:"userID"`
}

func handleTripPreview(w http.ResponseWriter, r *http.Request) {
	ctx, span := getTracer().Start(r.Context(), "handleTripPreview")
	defer span.End()

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

	tripPreview, err := tripService.Client.PreviewTrip(ctx, reqBody.toProto())
	if err != nil {
		log.Printf("Failed to preview a trip: %v", err)
		http.Error(w, "Failed to preview trip", http.StatusInternalServerError)
		return
	}

	response := contracts.APIResponse{Data: tripPreview}
	writeJSON(w, http.StatusCreated, response)
}

func handleTripStart(w http.ResponseWriter, r *http.Request) {
	ctx, span := getTracer().Start(r.Context(), "handleTripStart")
	defer span.End()

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

	resp, err := tripService.Client.CreateTrip(ctx, &pb.CreateTripRequest{
		RideFareID: reqBody.RideFareID,
		UserID:     reqBody.UserID,
	})
	if err != nil {
		st, ok := status.FromError(err)
		if ok {
			writeJSON(w, http.StatusInternalServerError, st.Message())
		} else {
			log.Printf("Failed to create trip: %v", err)
			writeJSON(w, http.StatusInternalServerError, "failed to create trip")
		}
		return
	}

	writeJSON(w, http.StatusOK, contracts.APIResponse{Data: resp})
}

func handleStripeWebhook(w http.ResponseWriter, r *http.Request, rb *messaging.RabbitMQ) {
	ctx, span := getTracer().Start(r.Context(), "handleStripeWebhook")
	defer span.End()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	webhookKey := env.GetString("STRIPE_WEBHOOK_KEY", "")
	if webhookKey == "" {
		log.Printf("Webhook key is required")
		return
	}

	event, err := webhook.ConstructEventWithOptions(
		body,
		r.Header.Get("Stripe-Signature"),
		webhookKey,
		webhook.ConstructEventOptions{
			IgnoreAPIVersionMismatch: true,
		},
	)
	if err != nil {
		log.Printf("Error verifying webhook signature: %v", err)
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	log.Printf("Received Stripe event: %v", event)

	switch event.Type {
	case "checkout.session.completed":
		var session stripe.CheckoutSession

		err := json.Unmarshal(event.Data.Raw, &session)
		if err != nil {
			log.Printf("Error parsing webhook JSON: %v", err)
			http.Error(w, "Invalid payload", http.StatusBadRequest)
			return
		}

		payload := messaging.PaymentStatusUpdateData{
			TripID:   session.Metadata["trip_id"],
			UserID:   session.Metadata["user_id"],
			DriverID: session.Metadata["driver_id"],
		}

		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			log.Printf("Error marshalling payload: %v", err)
			http.Error(w, "Failed to marshal payload", http.StatusInternalServerError)
			return
		}

		message := contracts.AmqpMessage{
			OwnerID: session.Metadata["user_id"],
			Data:    payloadBytes,
		}

		if err := rb.PublishMessage(
			ctx,
			contracts.PaymentEventSuccess,
			message,
		); err != nil {
			log.Printf("Error publishing payment event: %v", err)
			http.Error(w, "Failed to publish payment event", http.StatusInternalServerError)
			return
		}
	}
}
