package middleware

import (
	"strings"

	"example.com/m/v2/internal/common"
	"github.com/gin-gonic/gin"
)

const requestIDKey = "requestId"

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestID := strings.TrimSpace(c.GetHeader("X-Request-ID"))
		if requestID == "" {
			requestID = common.GenerateRequestID()
		}

		c.Set(requestIDKey, requestID)
		c.Header("X-Request-ID", requestID)
		c.Next()
	}
}

func GetRequestID(c *gin.Context) string {
	if requestID, ok := c.Get(requestIDKey); ok {
		if value, ok := requestID.(string); ok && value != "" {
			return value
		}
	}

	requestID := common.GenerateRequestID()
	c.Set(requestIDKey, requestID)
	c.Header("X-Request-ID", requestID)
	return requestID
}
