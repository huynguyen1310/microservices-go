package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"ride-sharing/services/trip-service/internal/infrastructure/events"
	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"
	"ride-sharing/shared/env"
	"ride-sharing/shared/messaging"

	grpc "ride-sharing/services/trip-service/internal/grpc"

	grpcserver "google.golang.org/grpc"
)

func main() {
	log.Println("Starting Trip Service")

	rabbitmqURL := env.GetString("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	inmemRepo := repository.NewInmemRepository()
	svc := service.NewService(inmemRepo)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh
		cancel()
	}()

	rabbitmq, err := messaging.NewRabbitMQ(rabbitmqURL)
	if err != nil {
		log.Fatalf("Failed to connect to RabbitMQ: %s", err)
	}
	defer rabbitmq.Close()
	log.Println("Connected to RabbitMQ")

	publisher := events.NewTripPublisher(rabbitmq)

	var GrpcAddr = ":9093"
	lis, err := net.Listen("tcp", GrpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grcpServer := grpcserver.NewServer()
	grpc.NewGRPCHandler(grcpServer, svc, publisher)

	log.Printf("Starting gRPC server Trip services on port %s", lis.Addr().String())

	go func() {
		if err := grcpServer.Serve(lis); err != nil {
			log.Fatalf("Failed to serve: %v", err)
			cancel()
		}
	}()

	<-ctx.Done()
	log.Printf("shutting down the server...")
	grcpServer.GracefulStop()

}
