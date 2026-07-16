package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	h "ride-sharing/services/trip-service/internal/infrastructure/http"
	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"
)

func main() {
	log.Println("Starting Trip Service")

	inmemRepo := repository.NewInmemRepository()
	svc := service.NewService(inmemRepo)
	mux := http.NewServeMux()

	httpHandler := h.HttpHandler{Service: svc}

	mux.HandleFunc("POST /preview", httpHandler.HandleTripPreview)

	server := &http.Server{
		Addr:    ":8083",
		Handler: mux,
	}

	serverError := make(chan error, 1)

	go func() {
		log.Printf("server listening on %s", server.Addr)
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

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Could not shutdown the server: %v", err)
		server.Close()
	}
}
