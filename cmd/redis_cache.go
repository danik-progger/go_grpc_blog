package server

import (
	"context"
	"encoding/json"
	"go_grpc_blog/db"

	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UpdateCache(s *Server, ctx context.Context) error {
	var dbPosts []db.Post
	result := s.Sql_DB.Preload("Author").Order("created_at desc").Limit(10).Offset(0).Find(&dbPosts)
	if result.Error != nil {
		return status.Errorf(codes.Internal, "failed to fetch posts: %v", result.Error)
	}

	s.Redis_DB.HSet(ctx, "posts_limit=10_offset=0", result, 0)
	if err := s.Redis_DB.HSet(ctx, "posts_limit=10_offset=0", result, 0).Err(); err != nil {
		return status.Errorf(codes.Internal, "failed to update cache")
	}

	return nil
}

func GetCachedPosts(s *Server, ctx context.Context) ([]db.Post, error) {
	var dbPosts []db.Post
	cachedData, err := s.Redis_DB.HGet(ctx, "posts", "posts_limit=10_offset=0").Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil
		}
		return nil, status.Errorf(codes.Internal, "failed to get posts from cache: %v", err)
	}

	if err := json.Unmarshal([]byte(cachedData), &dbPosts); err != nil {
		return nil, status.Errorf(codes.Internal, "failed to unmarshal cached posts: %v", err)
	}

	return dbPosts, nil
}
