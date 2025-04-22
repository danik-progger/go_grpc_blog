package db

import (
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func InitDB(link string) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(link), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	err = db.AutoMigrate(&User{}, &Post{})
	if err != nil {
		return nil, fmt.Errorf("failed to migrate models: %w", err)
	}

	if err := seedDefaultData(db); err != nil {
		return nil, fmt.Errorf("failed to seed default data: %w", err)
	}

	return db, nil
}

func seedDefaultData(db *gorm.DB) error {
	var userCount int64
	if err := db.Model(&User{}).Count(&userCount).Error; err != nil {
		return err
	}

	if userCount > 0 {
		return nil
	}

	tx := db.Begin()
	if tx.Error != nil {
		return tx.Error
	}

	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	usersMap := GetUsers()
	dbUsers := make(map[string]User)

	for id, apiUser := range usersMap {
		user := User{
			ID:       apiUser.Id,
			NickName: apiUser.NickName,
			PhotoURL: apiUser.PhotoUrl,
		}

		if err := tx.Create(&user).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create user %s: %w", id, err)
		}

		dbUsers[id] = user
	}

	for _, apiPost := range GetPosts() {
		createdAt, err := time.Parse("15:04:05 02.01.2006", apiPost.CreatedAt)
		if err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to parse date for post %s: %w", apiPost.Id, err)
		}

		post := Post{
			ID:        apiPost.Id,
			AuthorID:  apiPost.Author.Id,
			Body:      apiPost.Body,
			CreatedAt: createdAt,
		}

		if err := tx.Create(&post).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to create post %s: %w", apiPost.Id, err)
		}
	}

	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}
