package server

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	blog "go_grpc_blog/api"
	"go_grpc_blog/db"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func formatTimestamp(isoTimestamp string) (string, error) {
	t, err := time.Parse(time.RFC3339, isoTimestamp)
	if err != nil {
		return "", err
	}

	formatted := t.Format("15:04:05 02.01.2006")
	return formatted, nil
}

func sortPostsByCreatedAt(posts []*blog.Post, ascending bool) {
	sort.Slice(posts, func(i, j int) bool {
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
	Sql_DB *gorm.DB
	// Posts  []*blog.Post
	// Users  map[string]*blog.User
}

func dbPostToProtoPost(dbPost *db.Post, db *gorm.DB, userID string) *blog.Post {
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

	// Convert db.Post to blog.Post
	posts := make([]*blog.Post, len(dbPosts))
	for i, p := range dbPosts {
		posts[i] = dbPostToProtoPost(&p, s.Sql_DB, userID)
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
	protoPost := dbPostToProtoPost(&newPost, s.Sql_DB, authorID)

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

	protoPost := dbPostToProtoPost(&dbPost, s.Sql_DB, currentUserID)
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

// func (s *Server) ToggleLike(ctx context.Context, req *blog.ToggleLikeRequest) (*blog.ToggleLikeResponse, error) {
// 	md, ok := metadata.FromIncomingContext(ctx)
// 	if !ok {
// 		return nil, fmt.Errorf("missing user id")
// 	}
// 	userIDs := md.Get("user-id")
// 	if len(userIDs) == 0 {
// 		return nil, fmt.Errorf("user-id header is required")
// 	}

// 	// Find the post in the database
// 	var dbPost db.Post
// 	result := s.Sql_DB.Preload("Author").First(&dbPost, "id = ?", req.PostId)
// 	if result.Error != nil {
// 		return nil, fmt.Errorf("post not found: %v", result.Error)
// 	}

// 	// Check if the post is already liked by this user
// 	var postLike db.PostLike
// 	var isLiked bool
// 	result = s.Sql_DB.Where("post_id = ? AND user_id = ?", req.PostId, userID).First(&postLike)
// 	if result.Error == nil {
// 		// Like exists, remove it
// 		isLiked = false
// 		result = s.Sql_DB.Delete(&postLike)
// 		if result.Error != nil {
// 			return nil, fmt.Errorf("failed to remove like: %v", result.Error)
// 		}
// 	} else {
// 		// Like doesn't exist, add it
// 		isLiked = true
// 		postLike = db.PostLike{
// 			PostID: req.PostId,
// 			UserID: userID,
// 		}
// 		result = s.Sql_DB.Create(&postLike)
// 		if result.Error != nil {
// 			return nil, fmt.Errorf("failed to add like: %v", result.Error)
// 		}
// 	}

// 	// Get updated post with like count
// 	protoPost := dbPostToProtoPost(&dbPost, s.Sql_DB, userID)

// 	// Get likes count
// 	var likesCount int64
// 	s.Sql_DB.Model(&db.PostLike{}).Where("post_id = ?", req.PostId).Count(&likesCount)
// 	protoPost.LikesCount = int32(likesCount)
// 	protoPost.IsLiked = isLiked

// 	return &blog.ToggleLikeResponse{Post: protoPost}, nil
// }
