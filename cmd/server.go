package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	blog "go_grpc_blog/api"
	"go_grpc_blog/db"

	"github.com/go-redis/redis/v8"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Server struct {
	blog.UnimplementedBlogServiceServer
	Logger         *zap.Logger
	TimeToGetPosts *prometheus.HistogramVec
	Offset         prometheus.Histogram
	Limit          prometheus.Histogram
	TimeForRequest *prometheus.HistogramVec
	Sql_DB         *gorm.DB
	Redis_DB       *redis.Client
}

func NewServer(sqlDB *gorm.DB, redisAddr string) *Server {
	rdb := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	return &Server{
		Sql_DB:   sqlDB,
		Redis_DB: rdb,
	}
}

func dbPostToProtoPost(dbPost *db.Post, userID string) *blog.Post {
	return &blog.Post{
		Id: dbPost.ID,
		Author: &blog.User{
			Id:       dbPost.Author.ID,
			NickName: dbPost.Author.NickName,
			PhotoUrl: dbPost.Author.PhotoURL,
		},
		Body:      dbPost.Body,
		CreatedAt: dbPost.CreatedAt.Format("15:04:05 02.01.2006"),
	}
}

func (s *Server) GetPosts(ctx context.Context, req *blog.GetPostsRequest) (*blog.GetPostsResponse, error) {
	startReq := time.Now()
	s.Logger.Info("Request", zap.String("to", "GetPosts"))
	s.Logger.Info("Params", zap.Int32("limit", req.Limit))
	s.Logger.Info("Params", zap.Int32("offset", req.Offset))

	s.Limit.Observe(float64(req.Limit))
	s.Limit.Observe(float64(req.Offset))

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	headers := md.Get("user-id")
	if len(headers) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	userID := headers[0]
	s.Logger.Info("Params", zap.String("authorId", userID))

	var dbPosts []db.Post
	result := s.Sql_DB.Preload("Author").Order("created_at desc").Limit(int(req.Limit)).Offset(int(req.Offset)).Find(&dbPosts)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch posts: %v", result.Error)
	}

	posts := make([]*blog.Post, len(dbPosts))

	pipe := s.Redis_DB.Pipeline()
	likeCmds := make([]*redis.StringCmd, len(dbPosts))
	userLikeCmds := make([]*redis.StringCmd, len(dbPosts))

	for i, p := range dbPosts {
		postLikesKey := "post:" + p.ID + ":likes"
		likeCmds[i] = pipe.HGet(ctx, postLikesKey, "total-likes")
		userLikeCmds[i] = pipe.HGet(ctx, postLikesKey, userID)
	}

	s.Logger.Info("Redis before request", zap.String("from", "GetPosts"))
	s.Logger.Info("Redis before request", zap.String("wants to", "get posts and their likes"))
	start := time.Now()
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch likes: %v", err)
	}
	duration := time.Since(start).Seconds()
	s.TimeToGetPosts.WithLabelValues("GetPosts", "get liked by user and total likes for all posts").Observe(duration)
	s.Logger.Info("Redis after request", zap.String("result", "success"))

	for i, p := range dbPosts {
		totalLikes, err := likeCmds[i].Int64()
		if err != nil && err != redis.Nil {
			totalLikes = 0
		}
		if err == redis.Nil {
			s.Logger.Info("Redis before request", zap.String("from", "GetPosts"))
			s.Logger.Info("Redis before request", zap.String("wants to", "set post total likes"))
			start := time.Now()
			if err := s.Redis_DB.HSet(ctx, "post:"+p.ID+":likes", "total-likes", 0).Err(); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to initialize likes count for post %s: %v", p.ID, err)
			}
			duration := time.Since(start).Seconds()
			s.TimeToGetPosts.WithLabelValues("GetPosts").Observe(duration)
			s.Logger.Info("Redis after request", zap.String("result", "success"))
		}

		isLikedStr, err := userLikeCmds[i].Result()
		isLiked := err == nil && isLikedStr == "1"

		post := dbPostToProtoPost(&p, userID)
		post.LikesCount = int32(totalLikes)
		post.IsLiked = isLiked
		posts[i] = post
	}

	jsonPosts, err := json.MarshalIndent(posts, " ", " ")
	if err != nil {
		log.Fatal(err)
	}
	s.Logger.Info("Success", zap.ByteString("posts", jsonPosts))
	durationReq := time.Since(startReq).Seconds()
	s.TimeForRequest.WithLabelValues("GetPosts").Observe(durationReq)
	return &blog.GetPostsResponse{Posts: posts}, nil
}

func (s *Server) CreatePost(ctx context.Context, req *blog.CreatePostRequest) (*blog.CreatePostResponse, error) {
	startReq := time.Now()
	s.Logger.Info("Request", zap.String("to", "CreatePost"))
	s.Logger.Info("Params", zap.String("body", req.Body))

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	authorID := userIDs[0]
	s.Logger.Info("Params", zap.String("authorId", authorID))

	var user db.User
	result := s.Sql_DB.First(&user, "id = ?", authorID)
	if result.Error != nil {
		return nil, status.Errorf(codes.NotFound, "user not found: %v", result.Error)
	}

	newPost := db.Post{
		ID:        fmt.Sprintf("post-%d", time.Now().UnixNano()),
		Author:    user,
		Body:      req.Body,
		CreatedAt: time.Now(),
	}

	result = s.Sql_DB.Create(&newPost)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to create post: %v", result.Error)
	}

	s.Sql_DB.Preload("Author").First(&newPost, "id = ?", newPost.ID)
	protoPost := dbPostToProtoPost(&newPost, authorID)

	jsonPost, err := json.MarshalIndent(protoPost, " ", " ")
	if err != nil {
		log.Fatal(err)
	}
	s.Logger.Info("Success", zap.ByteString("post", jsonPost))
	durationReq := time.Since(startReq).Seconds()
	s.TimeForRequest.WithLabelValues("CreatePost").Observe(durationReq)
	return &blog.CreatePostResponse{Post: protoPost}, nil
}

func (s *Server) UpdatePost(ctx context.Context, req *blog.UpdatePostRequest) (*blog.UpdatePostResponse, error) {
	startReq := time.Now()
	s.Logger.Info("Request", zap.String("to", "UpdatePost"))
	s.Logger.Info("Params", zap.String("PostId", req.Id))
	s.Logger.Info("Params", zap.String("body", req.Body))

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	currentUserID := userIDs[0]
	s.Logger.Info("Params", zap.String("authorId", currentUserID))

	var dbPost db.Post
	result := s.Sql_DB.Preload("Author").First(&dbPost, "id = ?", req.Id)
	if result.Error != nil {
		return nil, status.Errorf(codes.NotFound, "post not found: %v", result.Error)
	}

	if dbPost.Author.ID != currentUserID {
		return nil, status.Error(codes.PermissionDenied, "only author can update the post")
	}

	dbPost.Body = req.Body
	dbPost.CreatedAt = time.Now()
	result = s.Sql_DB.Save(&dbPost)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to update post: %v", result.Error)
	}

	protoPost := dbPostToProtoPost(&dbPost, currentUserID)
	jsonPost, err := json.MarshalIndent(protoPost, " ", " ")
	if err != nil {
		log.Fatal(err)
	}
	s.Logger.Info("Success", zap.ByteString("post", jsonPost))
	durationReq := time.Since(startReq).Seconds()
	s.TimeForRequest.WithLabelValues("UpdatePost").Observe(durationReq)
	return &blog.UpdatePostResponse{Post: protoPost}, nil
}

func (s *Server) DeletePost(ctx context.Context, req *blog.DeletePostRequest) (*blog.DeletePostResponse, error) {
	startReq := time.Now()
	s.Logger.Info("Request", zap.String("to", "DeletePost"))
	s.Logger.Info("Params", zap.String("PostId", req.Id))

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	currentUserID := userIDs[0]
	s.Logger.Info("Params", zap.String("authorId", currentUserID))

	var dbPost db.Post
	result := s.Sql_DB.Preload("Author").First(&dbPost, "id = ?", req.Id)
	if result.Error != nil {
		return nil, status.Errorf(codes.NotFound, "post not found: %v", result.Error)
	}

	if dbPost.Author.ID != currentUserID {
		return nil, status.Error(codes.PermissionDenied, "only author can delete the post")
	}

	result = s.Sql_DB.Delete(&dbPost)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to delete post: %v", result.Error)
	}

	s.Logger.Info("Success", zap.String("deleted", dbPost.ID))
	durationReq := time.Since(startReq).Seconds()
	s.TimeForRequest.WithLabelValues("DeletePost").Observe(durationReq)
	return &blog.DeletePostResponse{}, nil
}

func (s *Server) ToggleLike(ctx context.Context, req *blog.ToggleLikeRequest) (*blog.ToggleLikeResponse, error) {
	startReq := time.Now()
	s.Logger.Info("Request", zap.String("to", "ToggleLike"))
	s.Logger.Info("Params", zap.String("PostId", req.PostId))

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	userID := userIDs[0]
	s.Logger.Info("Params", zap.String("authorId", userID))

	var dbPost db.Post
	result := s.Sql_DB.Preload("Author").First(&dbPost, "id = ?", req.PostId)
	if result.Error != nil {
		return nil, status.Errorf(codes.NotFound, "post not found: %v", result.Error)
	}

	postLikesKey := "post:" + req.PostId + ":likes"

	s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
	s.Logger.Info("Redis before request", zap.String("wants to", "find if user liked post"))
	start := time.Now()
	isLiked, err := s.Redis_DB.HGet(ctx, postLikesKey, userID).Bool()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to check like status: %v", err)
	}
	duration := time.Since(start).Seconds()
	s.TimeToGetPosts.WithLabelValues("ToggleLike", "get isLiked by user").Observe(duration)
	s.Logger.Info("Redis after request", zap.String("result", "success"))

	if isLiked {
		s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
		s.Logger.Info("Redis before request", zap.String("wants to", "delete like"))
		if err := s.Redis_DB.HDel(ctx, postLikesKey, userID).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove like from Redis: %v", err)
		}
		s.Logger.Info("Redis after request", zap.String("result", "success"))

		s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
		s.Logger.Info("Redis before request", zap.String("wants to", "decrement total likes"))
		if err := s.Redis_DB.HIncrBy(ctx, postLikesKey, "total-likes", -1).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to decrement total likes: %v", err)
		}
		s.Logger.Info("Redis after request", zap.String("result", "success"))

		isLiked = false
	} else {
		s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
		s.Logger.Info("Redis before request", zap.String("wants to", "set like"))
		if err := s.Redis_DB.HSet(ctx, postLikesKey, userID, true).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to add like to Redis: %v", err)
		}
		s.Logger.Info("Redis after request", zap.String("result", "success"))

		s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
		s.Logger.Info("Redis before request", zap.String("wants to", "increment total likes"))
		if err := s.Redis_DB.HIncrBy(ctx, postLikesKey, "total-likes", 1).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to increment total likes: %v", err)
		}
		s.Logger.Info("Redis after request", zap.String("result", "success"))

		isLiked = true
	}

	protoPost := dbPostToProtoPost(&dbPost, userID)

	s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
	s.Logger.Info("Redis before request", zap.String("wants to", "get total likes"))
	totalLikes, err := s.Redis_DB.HGet(ctx, postLikesKey, "total-likes").Int64()
	start = time.Now()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get total likes: %v", err)
	}
	duration = time.Since(start).Seconds()
	s.TimeToGetPosts.WithLabelValues("ToggleLike", "get total likes").Observe(duration)
	s.Logger.Info("Redis after request", zap.String("result", "success"))

	if err == redis.Nil {
		s.Logger.Info("Redis before request", zap.String("from", "ToggleLike"))
		s.Logger.Info("Redis before request", zap.String("wants to", "set total likes"))
		pipe := s.Redis_DB.Pipeline()
		pipe.HSet(ctx, postLikesKey, "total-likes", totalLikes)
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to initialize Redis likes: %v", err)
		}
		s.Logger.Info("Redis after request", zap.String("result", "success"))
	}

	protoPost.LikesCount = int32(totalLikes)
	protoPost.IsLiked = isLiked

	s.Logger.Info("Success", zap.Bool("liked", isLiked))

	durationReq := time.Since(startReq).Seconds()
	s.TimeForRequest.WithLabelValues("ToggleLike").Observe(durationReq)
	return &blog.ToggleLikeResponse{Post: protoPost}, nil
}
