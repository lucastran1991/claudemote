package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/mac/claudemote/backend/pkg/response"
	"github.com/mac/claudemote/backend/pkg/token"
)

// JWTAuthWithQuery is identical to JWTAuth but also accepts the JWT via the
// ?token= query parameter as a fallback. This is ONLY applied to the SSE stream
// route because the browser EventSource API cannot set custom HTTP headers.
//
// Security note: query-param tokens appear in server logs and browser history.
// Callers should use short-lived tokens or prefer the Authorization header when
// a non-EventSource client (e.g. curl) is used.
func JWTAuthWithQuery(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		raw := extractToken(c)
		if raw == "" {
			response.Error(c, http.StatusUnauthorized, "authorization required")
			c.Abort()
			return
		}

		claims, err := token.Validate(raw, jwtSecret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, "invalid or expired token")
			c.Abort()
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("username", claims.Username)
		c.Next()
	}
}

// extractToken tries the Authorization header first, then ?token= query param.
func extractToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) == 2 && parts[0] == "Bearer" {
			return parts[1]
		}
	}
	// Fallback for EventSource clients that cannot set headers.
	return c.Query("token")
}
