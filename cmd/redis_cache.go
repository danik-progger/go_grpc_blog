package server

import (
	"context"
	"encoding/json"
	"fmt"
	"go_grpc_blog/db"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UpdateCache(s *Server, ctx context.Context) error {
	var dbPosts []db.Post
	result := s.Sql_DB.Preload("Author").Order("created_at desc").Limit(10).Offset(0).Find(&dbPosts)
	if result.Error != nil {
		return status.Errorf(codes.Internal, "failed to fetch posts: %v", result.Error)
	}

	postsJSON, err := json.Marshal(dbPosts)
	if err != nil {
		return fmt.Errorf("failed to marshal posts: %v", err)
	}

	err = s.Redis_DB.Set(ctx, "posts_cache", postsJSON, 2*time.Minute).Err()
	if err != nil {
		return fmt.Errorf("failed to set cache: %v", err)
	}

	return nil
}


func GetCachedPosts(s *Server, ctx context.Context) ([]db.Post, error) {
	val, err := s.Redis_DB.Get(ctx, "posts_cache").Result()
	if err != nil {
		return nil, nil
	}

	var posts []db.Post
	if err := json.Unmarshal([]byte(val), &posts); err != nil {
		return nil, fmt.Errorf("failed to unmarshal posts: %v", err)
	}

	return posts, nil
}
