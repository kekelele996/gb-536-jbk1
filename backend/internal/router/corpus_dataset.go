package router

import (
	"github.com/gin-gonic/gin"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/handler"
	"corpus-annotation-agreement-control/backend/internal/middleware"
)

func registerCorpusDatasetRoutes(group *gin.RouterGroup, target *handler.CorpusDatasetHandler) {
	routes := group.Group("/datasets")
	routes.GET("", target.List)
	routes.GET("/:id", target.Get)
	write := routes.Group("")
	write.Use(middleware.RBAC(constants.RoleDataManager, constants.RoleAdmin))
	write.POST("", target.Create)
	write.PUT("/:id", target.Update)
	write.POST("/:id/transition", target.Transition)
}
