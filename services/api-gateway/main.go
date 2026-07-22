package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"
	"ride-sharing/shared/tracing"
)

var (
	httpAddr    = env.GetString("HTTP_ADDR", ":8081")
	rabbitmqURL = env.GetString("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
)

func main() {
	log.Println("Starting API Gateway")

	// Initialize Tracing
	tracerCfg := tracing.Config{
		ServiceName:    "api-gateway",
		Environment:    env.GetString("ENVIRONMENT", "development"),
		JaegerEndpoint: env.GetString("JAEGER_ENDPOINT", "http://jaeger:14268/api/traces"),
	}

	sh, err := tracing.InitTracer(tracerCfg)
	if err != nil {
		log.Fatalf("Failed to initialize the tracer: %v", err)
	}

	rabbitmq, err := messaging.NewRabbitMQ(rabbitmqURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %s", err)
	}
	defer rabbitmq.Close()
	log.Println("Connected to RabbitMQ")

	mux := http.NewServeMux()

	mux.Handle("POST /trip/preview", tracing.WrapHandlerFunc(enableCORS(handleTripPreview), "/trip/preview"))
	mux.Handle("POST /trip/start", tracing.WrapHandlerFunc(enableCORS(handleTripStart), "/trip/start"))
	mux.Handle("/ws/drivers", tracing.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleDriversWebSocket(w, r, rabbitmq)
	}, "/ws/drivers"))
	mux.Handle("/ws/riders", tracing.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleRidersWebSocket(w, r, rabbitmq)
	}, "/ws/riders"))
	mux.Handle("/webhook/stripe", tracing.WrapHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleStripeWebhook(w, r, rabbitmq)
	}, "/webhook/stripe"))

	server := &http.Server{
		Addr:    httpAddr,
		Handler: enableCORS(mux.ServeHTTP),
	}

	serverError := make(chan error, 1)

	go func() {
		log.Printf("server listening on %s", httpAddr)
		serverError <- server.ListenAndServe()
	}()

	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)

	select {
	case sig := <-shutdown:
		log.Printf("Server is shutting down due to %v signal", sig)
	case err := <-serverError:
		log.Printf("Error starting the server: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	defer sh(ctx)

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Could not shutdown the server: %v", err)
		server.Close()
	}
}
