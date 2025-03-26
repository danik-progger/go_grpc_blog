package server

import (
	"context"
	"fmt"
	"time"
	"sort"
	"strings"

	blog "go_grpc_blog/api"

	"google.golang.org/grpc/metadata"
)

func formatTimestamp(isoTimestamp string) (string, error) {
	t, err := time.Parse(time.RFC3339, isoTimestamp)
	if err != nil {
		return "", err
	}

	formatted := t.Format("15:04:05 02.01.2006")
	return formatted, nil
}

func sortPostsByCreatedAAt(posts []*blog.Post, ascending bool) {
	sort.Slice(posts, func(i, j int) bool {
		// Parse time from "hh:mm:ss dd.mm.yyyy" format
		parseTime := func(s string) time.Time {
			parts := strings.Split(s, " ")
			if len(parts) != 2 {
				return time.Time{}
			}
			t, _ := time.Parse("15:04:05 02.01.2006", parts[0]+" "+parts[1])
			return t
		}

		timeI := parseTime(posts[i].CreatedAt)
		timeJ := parseTime(posts[j].CreatedAt)

		if ascending {
			return timeI.Before(timeJ)
		}
		return timeI.After(timeJ)
	})
}

type Server struct {
	blog.UnimplementedBlogServiceServer
	Posts []*blog.Post
	Users map[string]*blog.User
}

func (s *Server) GetPosts(ctx context.Context, req *blog.GetPostsRequest) (*blog.GetPostsResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("user-id header is required")
	}

	posts := make([]*blog.Post, 0)
	for i, p := range s.Posts {
		if int32(i) < req.Offset {
			continue
		}

		if int32(len(posts)) >= req.Limit {
			break
		}
		posts = append(posts, p)
	}

	sortPostsByCreatedAAt(posts, false)

	return &blog.GetPostsResponse{Posts: posts}, nil
}

func (s *Server) CreatePost(ctx context.Context, req *blog.CreatePostRequest) (*blog.CreatePostResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("user-id header is required")
	}
	authorID := userIDs[0]

	// Check if user exists
	if _, exists := s.Users[authorID]; !exists {
		return nil, fmt.Errorf("user not found")
	}

	time, err := formatTimestamp(time.Now().Format(time.RFC3339))
	if err != nil {
		return nil, err
	}

	newPost := &blog.Post{
		Id:         fmt.Sprintf("post-%d", len(s.Posts)+1),
		AuthorId:   authorID,
		Body:       req.Body,
		CreatedAt:  time,
		LikesCount: 0,
		IsLiked:    false,
	}

	s.Posts = append([]*blog.Post{newPost}, s.Posts...)

	return &blog.CreatePostResponse{Post: newPost}, nil
}

func (s *Server) UpdatePost(ctx context.Context, req *blog.UpdatePostRequest) (*blog.UpdatePostResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("user-id header is required")
	}
	currentUserID := userIDs[0]

	for _, post := range s.Posts {
		if post.Id == req.Id {
			// Check if current user is the author
			if post.AuthorId != currentUserID {
				return nil, fmt.Errorf("only author can update the post")
			}
			post.Body = req.Body
			return &blog.UpdatePostResponse{Post: post}, nil
		}
	}

	return nil, fmt.Errorf("post not found")
}

func (s *Server) DeletePost(ctx context.Context, req *blog.DeletePostRequest) (*blog.DeletePostResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("user-id header is required")
	}
	currentUserID := userIDs[0]

	for i, post := range s.Posts {
		if post.Id == req.Id {
			// Check if current user is the author
			if post.AuthorId != currentUserID {
				return nil, fmt.Errorf("only author can delete the post")
			}
			s.Posts = append(s.Posts[:i], s.Posts[i+1:]...)
			return &blog.DeletePostResponse{Success: true}, nil
		}
	}

	return nil, fmt.Errorf("post not found")
}

func (s *Server) ToggleLike(ctx context.Context, req *blog.ToggleLikeRequest) (*blog.ToggleLikeResponse, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, fmt.Errorf("missing user id")
	}
	userIDs := md.Get("user-id")
	if len(userIDs) == 0 {
		return nil, fmt.Errorf("user-id header is required")
	}

	for _, post := range s.Posts {
		if post.Id == req.PostId {
			// Mock like toggle
			if post.IsLiked {
				post.LikesCount--
				post.IsLiked = false
			} else {
				post.LikesCount++
				post.IsLiked = true
			}
			return &blog.ToggleLikeResponse{Post: post}, nil
		}
	}

	return nil, fmt.Errorf("post not found")
}
