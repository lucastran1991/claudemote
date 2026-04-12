package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Health handles GET /api/health.
// No auth required — used by load balancers and uptime monitors.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
