package db

import (
	blog "go_grpc_blog/api"
)

func GetUsers() map[string]*blog.User {
	return map[string]*blog.User {
		"user-1": {
			Id:       "user-1",
			NickName: "naruto_uzumaki",
			PhotoUrl: "https://naruto-photo.jpg",
		},
		"user-2": {
			Id:       "user-2",
			NickName: "tanjiro_kamada",
			PhotoUrl: "https://tanjiro-photo.jpg",
		},
		"user-3": {
			Id:       "user-3",
			NickName: "kilua_zoldyck",
			PhotoUrl: "https://kilua-photo.jpg",
		},
		"user-4": {
			Id:       "user-4",
			NickName: "satoru_gojo",
			PhotoUrl: "https://satoru-photo.jpg",
		},
	}
}

func GetPosts() []*blog.Post {
	return []*blog.Post{
		{
			Id:         "post-1",
			AuthorId:   "user-1",
			Body:       "Post 1 by Naruto!",
			CreatedAt:  "16:11:00 26.03.2025",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-2",
			AuthorId:   "user-2",
			Body:       "Post 2 by Tanjiro!",
			CreatedAt:  "12:00:00 13.01.2025",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-3",
			AuthorId:   "user-1",
			Body:       "Post 3 by Naruto!",
			CreatedAt:  "00:00:00 01.01.2025",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-4",
			AuthorId:   "user-4",
			Body:       "Post 4 by Satoru!",
			CreatedAt:  "20:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
	}
}
