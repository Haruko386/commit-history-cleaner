package internal

import (
	"example.com/m/v2/internal/handler"
	"github.com/gin-gonic/gin"
)

type Router struct {
	healthHandler *handler.HealthHandler
	repoHandler   *handler.RepositoriesHandler
}

func NewRouter(healthHandler *handler.HealthHandler, repoHandler *handler.RepositoriesHandler) *Router {
	return &Router{healthHandler: healthHandler, repoHandler: repoHandler}
}

func (r *Router) Setup(e *gin.Engine) {
	api := e.Group("/api/v1")
	{
		api.GET("/health", r.healthHandler.CheckHealth)
		api.GET("/github/connection", r.healthHandler.CheckGithubConnection)
		repositories := api.Group("/repositories")
		{
			repositories.POST("open", r.repoHandler.OpenRepository)
			repositories.GET("current", r.repoHandler.GetCurrentRepository)
			repositories.DELETE("current", r.repoHandler.ExitCurrentRepository)
			//repositories.GET("recent")
		}
	}
}
