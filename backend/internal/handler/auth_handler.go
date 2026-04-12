package handler

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/mac/claudemote/backend/internal/service"
	"github.com/mac/claudemote/backend/pkg/response"
)

// LoginRequest is the payload for POST /api/auth/login.
type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// AuthHandler handles authentication HTTP requests.
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates an AuthHandler wired to the given AuthService.
func NewAuthHandler(authService *service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}

// Login handles POST /api/auth/login.
// Returns a signed JWT on success; 401 on invalid credentials.
func (h *AuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}

	tok, err := h.authService.Login(req.Username, req.Password)
	if err != nil {
		if errors.Is(err, service.ErrInvalidCredentials) {
			response.Error(c, http.StatusUnauthorized, "invalid username or password")
			return
		}
		response.Error(c, http.StatusInternalServerError, "login failed")
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tok})
}
