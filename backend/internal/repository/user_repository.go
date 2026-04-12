package repository

import (
	"github.com/mac/claudemote/backend/internal/model"
	"gorm.io/gorm"
)

// UserRepository handles database operations for the User model.
type UserRepository struct {
	db *gorm.DB
}

// NewUserRepository creates a new UserRepository backed by db.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{db: db}
}

// Create inserts a new user row.
func (r *UserRepository) Create(user *model.User) error {
	return r.db.Create(user).Error
}

// FindByUsername retrieves a user by their unique username.
func (r *UserRepository) FindByUsername(username string) (*model.User, error) {
	var user model.User
	if err := r.db.Where("username = ?", username).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByID retrieves a user by primary key.
func (r *UserRepository) FindByID(id uint) (*model.User, error) {
	var user model.User
	if err := r.db.First(&user, id).Error; err != nil {
		return nil, err
	}
	return &user, nil
}
