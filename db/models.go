package db

import (
	"time"
)

type User struct {
	ID       string `gorm:"primaryKey"`
	NickName string `gorm:"size:100;unique;not null"`
	PhotoURL string
}

type Post struct {
	ID        string `gorm:"primaryKey"`
	AuthorID  string
	Author    User   `gorm:"foreignKey:AuthorID"`
	Body      string `gorm:"not null"`
	CreatedAt time.Time
}
