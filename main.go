package main

import (
	"log"
	"net"

	blog "go_grpc_blog/api"
	server "go_grpc_blog/cmd"
	db "go_grpc_blog/db"

	"google.golang.org/grpc"
)


func main() {
	s := &server.Server{
		Users: db.GetUsers(),
		Posts: db.GetPosts(),
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	blog.RegisterBlogServiceServer(grpcServer, s)

	log.Println("Starting gRPC server on :50051")
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
