package main

import (
	"context"
	"log" // TODO: Consider using slog.

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "go.creack.net/telepilot/api/v1"
)

func main() {
	conn, err := grpc.NewClient("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("grpc new client: %s.", err)
	}
	defer func() { _ = conn.Close() }() // Best effort.

	client := pb.NewTelePilotServiceClient(conn)

	ctx := context.Background()

	if _, err := client.StartJob(ctx, &pb.StartJobRequest{}); err != nil {
		log.Printf("Failed to start job: %s.", err)
	}
}
