package main

import (
	"context"
	"io/fs"
	"log"
	"net"
	"net/http"
	"time"

	"embed"

	blog "go_grpc_blog/api"
	server "go_grpc_blog/cmd"
	db "go_grpc_blog/db"

	"github.com/go-redis/redis/v8"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

//go:embed api/api.swagger.json
var swaggerData []byte

//go:embed swagger-ui
var swaggerFiles embed.FS

func main() {
	sql_db, err := db.InitDB("host=localhost dbname=postgres port=5432 sslmode=disable TimeZone=UTC")
	if err != nil {
		log.Fatalf("🔴 Failed to initialize sql database: %v", err)
	}
	log.Println("🟢 Starting SQL DB  on port 5432")

	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})

	ctx := context.Background()
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		log.Fatalf("🔴 Failed to initialize redis database: %v", err)
	}
	log.Println("🟢 Starting Redis DB on port 6379")

	s := &server.Server{
		Sql_DB:   sql_db,
		Redis_DB: rdb,
	}

	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()

		for {
			if err := server.UpdateCache(s, context.Background()); err != nil {
				log.Printf("🔴 Cache update error: %v", err)
			}
			log.Println("🟢 Cache successfully updated")
			<-ticker.C
		}
	}()

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("🔴 Failed to listen: %v", err)
	}

	grpcServer := grpc.NewServer()
	blog.RegisterBlogServiceServer(grpcServer, s)
	reflection.Register(grpcServer)

	go func() {
		log.Println("🟢 Starting gRPC server on :50051")
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("🔴 Failed to serve: %v", err)
		}
	}()

	gwmux := runtime.NewServeMux()
	conn, err := grpc.NewClient(":50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalln("🔴 Failed to dial server:", err)
	}
	blog.RegisterBlogServiceHandler(context.Background(), gwmux, conn)

	mux := http.NewServeMux()
	mux.Handle("/", gwmux)
	mux.HandleFunc("/swagger-ui/swagger.json", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(swaggerData)
	})

	fSys, err := fs.Sub(swaggerFiles, "swagger-ui")
	if err != nil {
		panic(err)
	}

	mux.Handle("/swagger-ui/", http.StripPrefix("/swagger-ui/", http.FileServer(http.FS(fSys))))

	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: mux,
	}

	log.Println("🟢 Serving gRPC-Gateway on http://0.0.0.0:8090")
	log.Fatalln(gwServer.ListenAndServe())
}
