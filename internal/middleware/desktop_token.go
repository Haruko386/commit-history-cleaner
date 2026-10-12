package middleware

import (
	"net/http"

	"github.com/Haruko386/commit-history-cleaner/internal/common"
	"github.com/gin-gonic/gin"
)

func DesktopToken(expected string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if expected == "" {
			c.Next()
			return
		}
		if c.GetHeader(common.DesktopTokenHeader) != expected {
			c.AbortWithStatusJSON(http.StatusUnauthorized, common.NewErrorResponse(common.Unauthorized, "unauthorized desktop request。", false, GetRequestID(c)))
			return
		}
		c.Next()
	}
}
