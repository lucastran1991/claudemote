package response

import "github.com/gin-gonic/gin"

// OK sends a 200 JSON response with data wrapped under the "data" key.
func OK(c *gin.Context, data any) {
	c.JSON(200, gin.H{"data": data})
}

// Success sends a JSON response with the given status code and data.
func Success(c *gin.Context, status int, data any) {
	c.JSON(status, gin.H{"data": data})
}

// Error sends a JSON error response with the given status code and message.
func Error(c *gin.Context, status int, message string) {
	c.JSON(status, gin.H{"error": message})
}

// Paginated sends a paginated list response.
func Paginated(c *gin.Context, items any, total int64, page, pageSize int) {
	c.JSON(200, gin.H{
		"data":      items,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}
