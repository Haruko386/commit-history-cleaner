package internal

import (
	"example.com/m/v2/internal/handler"
	"github.com/gin-gonic/gin"
)

type Router struct {
	healthHandler *handler.HealthHandler
}

func NewRouter(healthHandler *handler.HealthHandler) *Router {
	return &Router{healthHandler: healthHandler}
}

func (r *Router) Setup(e *gin.Engine) {
	api := e.Group("/api/v1")
	{
		api.GET("/health", r.healthHandler.CheckHealth)
		api.GET("github/connection")
		//repositories := api.Group("/repositories")
		//{
		//
		//}
	}
}
