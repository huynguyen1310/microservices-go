package main

import (
	"context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"ride-sharing/services/trip-service/internal/infrastructure/repository"
	"ride-sharing/services/trip-service/internal/service"

	grpc "ride-sharing/services/trip-service/internal/grpc"

	grpcserver "google.golang.org/grpc"
)

func main() {
	log.Println("Starting Trip Service")

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

	var GrpcAddr = ":9093"
	lis, err := net.Listen("tcp", GrpcAddr)
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	grcpServer := grpcserver.NewServer()
	grpc.NewGRPCHandler(grcpServer, svc)

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
