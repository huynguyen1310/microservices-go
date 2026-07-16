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
)

var (
	httpAddr = env.GetString("HTTP_ADDR", ":8081")
)

func main() {
	log.Println("Starting API Gateway")

	mux := http.NewServeMux()

	mux.HandleFunc("POST /trip/preview", enableCORS(handleTripPreview))
	mux.HandleFunc("/ws/drivers", handleDriversWebsocket)
	mux.HandleFunc("/ws/riders", handleRidersWebsocket)

	server := &http.Server{
		Addr:    httpAddr,
		Handler: mux,
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

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Could not shutdown the server: %v", err)
		server.Close()
	}
}
