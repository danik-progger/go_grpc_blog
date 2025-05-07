package main

import (
	"context"
	"encoding/json"
	"fmt"
	blog "go_grpc_blog/api"
	server "go_grpc_blog/cmd"
	db "go_grpc_blog/db"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/go-redis/redismock/v8"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestGetPostsFromSqlDB(t *testing.T) {
	sql_db, mockDB, err := sqlmock.New()
	require.NoError(t, err)
	defer sql_db.Close()

	dialector := postgres.New(postgres.Config{
		Conn: sql_db,
	})

	gormDB, err := gorm.Open(dialector, &gorm.Config{})
	require.NoError(t, err)

	app := &server.Server{Sql_DB: gormDB}

	rows := sqlmock.NewRows([]string{"id", "author", "body", "created_at"}).
		AddRow("post-1", db.GetUserById("user-1"), "Post 1 by Naruto!", "13:11:00 26.03.2025").
		AddRow("post-2", db.GetUserById("user-2"), "Post 2 by Tanjiro!", "13:11:00 26.03.2025").
		AddRow("post-3", db.GetUserById("user-1"), "Post 3 by Naruto!", "13:11:00 26.03.2025").
		AddRow("post-4", db.GetUserById("user-4"), "Post 4 by Satoru!", "13:11:00 26.03.2025").
		AddRow("post-5", db.GetUserById("user-5"), "Post 5 by Katacura!", "13:11:00 26.03.2025").
		AddRow("post-6", db.GetUserById("user-6"), "Post 6 by Eren!", "13:11:00 26.03.2025").
		AddRow("post-7", db.GetUserById("user-7"), "Post 7 by Kaneki!", "13:11:00 26.03.2025").
		AddRow("post-8", db.GetUserById("user-8"), "Post 8 by Izuki!", "13:11:00 26.03.2025").
		AddRow("post-9", db.GetUserById("user-9"), "Post 9 by Ichigo!", "13:11:00 26.03.2025").
		AddRow("post-10", db.GetUserById("user-10"), "Post 10 by Goku!", "13:11:00 26.03.2025")

	mockDB.ExpectQuery(regexp.QuoteMeta(
		`SELECT * FROM "posts" ORDER BY created_at DESC LIMIT 7 OFFSET 2`)).
		WillReturnRows(rows)

	ctx := context.Background()
	// Non-default parameters
	getBody := blog.GetPostsRequest{
		Limit:  7,
		Offset: 2,
	}
	resp, err := app.GetPosts(ctx, &getBody)
	require.NoError(t, err)
	require.Len(t, resp, 5)
	require.Equal(t, "Post 1 by Naruto!", resp.Posts[0].Body)
	require.Equal(t, "Post 2 by Tanjiro!", resp.Posts[1].Body)

	require.NoError(t, mockDB.ExpectationsWereMet())
}

func TestGetValueFromRedis(t *testing.T) {
	redis, mockdb := redismock.NewClientMock()

	app := &server.Server{Redis_DB: redis}

	mockdb.ExpectGet("k").SetVal("v")
	dbPosts := db.GetPosts()
	postsJSON, err := json.Marshal(dbPosts)
	if err != nil {
		fmt.Printf("failed to marshal posts: %v", err)
	}
	mockdb.ExpectGet("k").SetVal(string(postsJSON))

	ctx := context.Background()
	getBody := blog.GetPostsRequest{
		Limit:  10,
		Offset: 0,
	}
	value, err := app.GetPosts(ctx, &getBody)
	require.NoError(t, err)
	require.Equal(t, string(postsJSON), value)

	require.NoError(t, mockdb.ExpectationsWereMet())
}
