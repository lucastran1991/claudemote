package model

import "time"

// User represents an admin account stored in the users table.
// Created out-of-band via cmd/create-admin — no signup endpoint.
type User struct {
	ID           uint      `gorm:"primaryKey"           json:"id"`
	Username     string    `gorm:"uniqueIndex;not null" json:"username"`
	PasswordHash string    `gorm:"not null"             json:"-"`
	CreatedAt    time.Time `                            json:"created_at"`
}
