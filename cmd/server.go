package server

import (
	"context"
	"fmt"
	"time"

	blog "go_grpc_blog/api"
	"go_grpc_blog/db"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

type Server struct {
	blog.UnimplementedBlogServiceServer
	Sql_DB   *gorm.DB
	Redis_DB *redis.Client
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
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	headers := md.Get("user-id")
	if len(headers) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	userID := headers[0]

	var dbPosts []db.Post
	result := s.Sql_DB.Preload("Author").Order("created_at desc").Limit(int(req.Limit)).Offset(int(req.Offset)).Find(&dbPosts)
	if result.Error != nil {
		return nil, status.Errorf(codes.Internal, "failed to fetch posts: %v", result.Error)
	}

	posts := make([]*blog.Post, len(dbPosts))
	for i, p := range dbPosts {
		postLikesKey := "post:" + p.ID + ":likes"

		totalLikes, err := s.Redis_DB.HGet(ctx, postLikesKey, "total-likes").Int64()
		if err != nil && err != redis.Nil {
			return nil, status.Errorf(codes.Internal, "failed to get likes count for post %s: %v", p.ID, err)
		}
		if err == redis.Nil {
			totalLikes = 0
			if err := s.Redis_DB.HSet(ctx, postLikesKey, "total-likes", totalLikes).Err(); err != nil {
				return nil, status.Errorf(codes.Internal, "failed to initialize likes count for post %s: %v", p.ID, err)
			}
		}

		isLiked, err := s.Redis_DB.HGet(ctx, postLikesKey, userID).Bool()
		if err != nil && err != redis.Nil {
			return nil, status.Errorf(codes.Internal, "failed to check like status for post %s: %v", p.ID, err)
		}
		if err == redis.Nil {
			isLiked = false
		}

		post := dbPostToProtoPost(&p, userID)
		post.LikesCount = int32(totalLikes)
		post.IsLiked = isLiked
		posts[i] = post
	}

	return &blog.GetPostsResponse{Posts: posts}, nil
}

func (s *Server) CreatePost(ctx context.Context, req *blog.CreatePostRequest) (*blog.CreatePostResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	authorID := userIDs[0]

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

	return &blog.CreatePostResponse{Post: protoPost}, nil
}

func (s *Server) UpdatePost(ctx context.Context, req *blog.UpdatePostRequest) (*blog.UpdatePostResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	currentUserID := userIDs[0]

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
	return &blog.UpdatePostResponse{Post: protoPost}, nil
}

func (s *Server) DeletePost(ctx context.Context, req *blog.DeletePostRequest) (*blog.DeletePostResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	currentUserID := userIDs[0]

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

	return &blog.DeletePostResponse{}, nil
}

func (s *Server) ToggleLike(ctx context.Context, req *blog.ToggleLikeRequest) (*blog.ToggleLikeResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, status.Error(codes.Unauthenticated, "user-id header is required")
	}
	userID := userIDs[0]

	var dbPost db.Post
	result := s.Sql_DB.Preload("Author").First(&dbPost, "id = ?", req.PostId)
	if result.Error != nil {
		return nil, status.Errorf(codes.NotFound, "post not found: %v", result.Error)
	}

	postLikesKey := "post:" + req.PostId + ":likes"

	isLiked, err := s.Redis_DB.HGet(ctx, postLikesKey, userID).Bool()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to check like status: %v", err)
	}

	if isLiked {
		if err := s.Redis_DB.HDel(ctx, postLikesKey, userID).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to remove like from Redis: %v", err)
		}
		if err := s.Redis_DB.HIncrBy(ctx, postLikesKey, "total-likes", -1).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to decrement total likes: %v", err)
		}
		isLiked = false
	} else {
		if err := s.Redis_DB.HSet(ctx, postLikesKey, userID, true).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to add like to Redis: %v", err)
		}
		if err := s.Redis_DB.HIncrBy(ctx, postLikesKey, "total-likes", 1).Err(); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to increment total likes: %v", err)
		}
		isLiked = true
	}

	protoPost := dbPostToProtoPost(&dbPost, userID)

	totalLikes, err := s.Redis_DB.HGet(ctx, postLikesKey, "total-likes").Int64()
	if err != nil && err != redis.Nil {
		return nil, status.Errorf(codes.Internal, "failed to get total likes: %v", err)
	}
	if err == redis.Nil {
		pipe := s.Redis_DB.Pipeline()
		pipe.HSet(ctx, postLikesKey, "total-likes", totalLikes)
		if _, err := pipe.Exec(ctx); err != nil {
			return nil, status.Errorf(codes.Internal, "failed to initialize Redis likes: %v", err)
		}
	}

	protoPost.LikesCount = int32(totalLikes)
	protoPost.IsLiked = isLiked

	return &blog.ToggleLikeResponse{Post: protoPost}, nil
}
