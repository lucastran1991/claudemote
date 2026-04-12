package service

import (
	"errors"

	"github.com/mac/claudemote/backend/internal/repository"
	"github.com/mac/claudemote/backend/pkg/token"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	// ErrInvalidCredentials is returned when username or password is wrong.
	ErrInvalidCredentials = errors.New("invalid username or password")
)

// AuthService handles login business logic.
// Admin accounts are created out-of-band via cmd/create-admin or env-based
// bcrypt hash seeding — no signup endpoint exists.
type AuthService struct {
	userRepo  *repository.UserRepository
	jwtSecret string
}

// NewAuthService creates an AuthService with the given repo and JWT secret.
func NewAuthService(repo *repository.UserRepository, jwtSecret string) *AuthService {
	return &AuthService{userRepo: repo, jwtSecret: jwtSecret}
}

// Login validates username + password against the stored bcrypt hash.
// Returns a signed JWT on success or ErrInvalidCredentials on failure.
func (s *AuthService) Login(username, password string) (string, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrInvalidCredentials
		}
		return "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", ErrInvalidCredentials
	}

	tok, err := token.Generate(user.ID, user.Username, s.jwtSecret)
	if err != nil {
		return "", err
	}
	return tok, nil
}
