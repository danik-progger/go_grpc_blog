package main

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"

	"embed"

	blog "go_grpc_blog/api"
	server "go_grpc_blog/cmd"
	db "go_grpc_blog/db"

	"github.com/go-redis/redis/v8"
	grpc_zap "github.com/grpc-ecosystem/go-grpc-middleware/logging/zap"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

//go:embed api/api.swagger.json
var swaggerData []byte

//go:embed swagger-ui
var swaggerFiles embed.FS

func main() {
	// Logger
	logger, err := zap.NewProduction()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	logger.Info("main.go", zap.String("🟢", "Logger initialized"))

	// SQL
	sql_db, err := db.InitDB("host=localhost dbname=postgres port=5432 sslmode=disable TimeZone=UTC")
	if err != nil {
		logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to initialize sql database: %v", err)))
	}
	logger.Info("main.go", zap.String("🟢", "Starting SQL DB  on port 5432"))

	// Redis
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:6379"})

	ctx := context.Background()
	_, err = rdb.Ping(ctx).Result()
	if err != nil {
		logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to initialize redis database: %v", err)))

	}
	logger.Info("main.go", zap.String("🟢", "Starting Redis DB on port 6379"))

	// Metrics
	// We can analyze histogram for each method and each operation
	timeToGetLike := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "Duration_of_requests_to_Redis_DB_in_seconds",
			Buckets: []float64{0.001, 0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"endpoint", "operation"},
	)
	prometheus.MustRegister(timeToGetLike)

	// Now we cache limit=10 offset=0. What if these boundaries are wrong and we
	// need to use different offset or different amount of posts to cache
	limit := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "Limit",
			Buckets: []float64{0, 5, 10, 15, 25, 50, 100},
		},
	)
	prometheus.MustRegister(limit)
	offset := prometheus.NewHistogram(
		prometheus.HistogramOpts{
			Name:    "Offset",
			Buckets: []float64{0, 5, 10, 15, 25, 50, 100},
		},
	)
	prometheus.MustRegister(offset)

	// Measure how much time it gets in total to perform a request to each endpoint
	timeForRequest := prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "Time_for_a_request",
			Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"endpoint"},
	)
	prometheus.MustRegister(timeForRequest)

	go func() {
		server := &http.Server{
			Addr:    ":9000",
			Handler: promhttp.Handler(),
		}

		logger.Info("main.go", zap.String("🟢", "Serving metrics on http://0.0.0.0:9000/metrics"))

		if err := server.ListenAndServe(); err != nil {
			logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to start metrics server: %v", err)))
		}
	}()

	// Init main server
	s := &server.Server{
		Logger:         logger,
		TimeToGetPosts: timeToGetLike,
		Limit:          limit,
		Offset:         offset,
		TimeForRequest: timeForRequest,
		Sql_DB:         sql_db,
		Redis_DB:       rdb,
	}

	// Start gRPC server
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to listen tcp server: %v", err)))
	}

	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			grpc_zap.UnaryServerInterceptor(logger),
			grpc_prometheus.UnaryServerInterceptor,
		),
	)
	blog.RegisterBlogServiceServer(grpcServer, s)
	reflection.Register(grpcServer)

	go func() {
		logger.Info("main.go", zap.String("🟢", "Starting gRPC server on :50051"))
		if err := grpcServer.Serve(lis); err != nil {
			logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to serve gRPC server: %v", err)))
		}
	}()

	gwmux := runtime.NewServeMux()
	conn, err := grpc.NewClient(":50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to dial server:: %v", err)))
	}
	blog.RegisterBlogServiceHandler(context.Background(), gwmux, conn)

	// Swagger
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

	// Gateway
	gwServer := &http.Server{
		Addr:    ":8090",
		Handler: mux,
	}

	logger.Info("main.go", zap.String("🟢", "Serving gRPC-Gateway on http://0.0.0.0:8090"))
	if err := gwServer.ListenAndServe(); err != nil {
		logger.Info("main.go", zap.String("🔴", fmt.Sprintf("Failed to serve gRPC-Gateway: %v", err)))
	}
}
