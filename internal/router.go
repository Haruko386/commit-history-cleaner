package internal

import (
	"github.com/Haruko386/commit-history-cleaner/internal/handler"
	"github.com/gin-gonic/gin"
)

type Router struct {
	healthHandler *handler.HealthHandler
	repoHandler   *handler.RepositoriesHandler
	taskHandler   *handler.TaskHandler
}

func NewRouter(
	healthHandler *handler.HealthHandler,
	repoHandler *handler.RepositoriesHandler,
	taskHandler *handler.TaskHandler,
) *Router {
	return &Router{healthHandler: healthHandler, repoHandler: repoHandler, taskHandler: taskHandler}
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
			repositories.POST("current/scans", r.repoHandler.ScanRepository)

			//repositories.GET("recent")
		}

		tasks := api.Group("/tasks")
		{
			tasks.GET("/:taskID", r.taskHandler.GetTask)
			tasks.DELETE("/:taskID", r.taskHandler.CancelTask)
		}
	}
}
