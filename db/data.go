package db

import (
	blog "go_grpc_blog/api"
)

func GetUsers() map[string]*blog.User {
	return map[string]*blog.User{
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
		"user-5": {
			Id:       "user-5",
			NickName: "katakura_ken",
			PhotoUrl: "https://katakura-photo.jpg",
		},
		"user-6": {
			Id:       "user-6",
			NickName: "eren_yeager",
			PhotoUrl: "https://eren-photo.jpg",
		},
		"user-7": {
			Id:       "user-7",
			NickName: "kaneki_ken",
			PhotoUrl: "https://eren-photo.jpg",
		},
		"user-8": {
			Id:       "user-8",
			NickName: "izuki_midoriya",
			PhotoUrl: "https://midoriya-photo.jpg",
		},
		"user-9": {
			Id:       "user-9",
			NickName: "ichigo_kurosaki",
			PhotoUrl: "https://ichigo-photo.jpg",
		},
		"user-10": {
			Id:       "user-10",
			NickName: "son_goku",
			PhotoUrl: "https://goku-photo.jpg",
		},
	}
}

func GetUserById(id string) *blog.User {
	return GetUsers()[id]
}

func GetPosts() []*blog.Post {
	return []*blog.Post{
		{
			Id:         "post-1",
			Author:     GetUserById("user-1"),
			Body:       "Post 1 by Naruto!",
			CreatedAt:  "13:11:00 26.03.2025",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-2",
			Author:     GetUserById("user-2"),
			Body:       "Post 2 by Tanjiro!",
			CreatedAt:  "09:00:00 13.01.2025",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-3",
			Author:     GetUserById("user-1"),
			Body:       "Post 3 by Naruto!",
			CreatedAt:  "21:00:00 31.12.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-4",
			Author:     GetUserById("user-4"),
			Body:       "Post 4 by Satoru!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-5",
			Author:     GetUserById("user-5"),
			Body:       "Post 5 by Katacura!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-6",
			Author:     GetUserById("user-6"),
			Body:       "Post 6 by Eren!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-7",
			Author:     GetUserById("user-7"),
			Body:       "Post 7 by Kaneki!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-8",
			Author:     GetUserById("user-8"),
			Body:       "Post 8 by Izuki!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-9",
			Author:     GetUserById("user-9"),
			Body:       "Post 9 by Ichigo!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
		{
			Id:         "post-10",
			Author:     GetUserById("user-10"),
			Body:       "Post 10 by Goku!",
			CreatedAt:  "17:31:00 03.09.2024",
			LikesCount: 0,
			IsLiked:    false,
		},
	}
}
