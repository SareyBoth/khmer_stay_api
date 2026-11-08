package response

import "github.com/gin-gonic/gin"

func JSON(c *gin.Context, status int, body any) { c.JSON(status, body) }
func Error(c *gin.Context, status int, message string) {
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
