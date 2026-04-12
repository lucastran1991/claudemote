package router

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/mac/claudemote/backend/internal/handler"
	"github.com/mac/claudemote/backend/internal/middleware"
)

// Setup wires all handlers, middleware, and routes into a *gin.Engine.
// Called once during server boot with fully-constructed handler instances.
func Setup(
	authHandler *handler.AuthHandler,
	jobHandler *handler.JobHandler,
	streamHandler *handler.StreamHandler,
	jwtSecret string,
	corsOrigin string,
) *gin.Engine {
	r := gin.Default()

	// CORS — allow the Next.js frontend origin in dev and prod.
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{corsOrigin},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	// Public routes — no JWT required.
	r.GET("/api/health", handler.Health)
	// Rate-limited: 5 attempts per minute per IP to prevent brute-force.
	r.POST("/api/auth/login",
		middleware.LoginRateLimit(5, time.Minute),
		authHandler.Login,
	)

	// Protected routes — standard JWT (Authorization header only).
	api := r.Group("/api")
	api.Use(middleware.JWTAuth(jwtSecret))
	{
		api.GET("/jobs", jobHandler.List)
		api.POST("/jobs", jobHandler.Enqueue)
		api.GET("/jobs/:id", jobHandler.Get)
		api.POST("/jobs/:id/cancel", jobHandler.Cancel)
	}

	// SSE stream route uses a separate middleware that also accepts ?token= query
	// param because the browser EventSource API cannot set custom headers.
	stream := r.Group("/api")
	stream.Use(middleware.JWTAuthWithQuery(jwtSecret))
	{
		stream.GET("/jobs/:id/stream", streamHandler.Stream)
	}

	return r
}
