package router

import (
	"github.com/gin-gonic/gin"

	"corpus-annotation-agreement-control/backend/internal/constants"
	"corpus-annotation-agreement-control/backend/internal/handler"
	"corpus-annotation-agreement-control/backend/internal/middleware"
)

func registerAnnotationSetRoutes(group *gin.RouterGroup, target *handler.AnnotationSetHandler) {
	routes := group.Group("/annotations")
	routes.GET("", target.List)
	routes.GET("/:id", target.Get)
	create := routes.Group("")
	create.Use(middleware.RBAC(constants.RoleAnnotator, constants.RoleAdmin))
	create.POST("", target.Create)
	create.PUT("/:id", target.Update)
	routes.POST("/:id/transition",
		middleware.RBAC(constants.RoleAnnotator, constants.RoleDataManager, constants.RoleAdjudicator, constants.RoleAdmin),
		middleware.RateLimit(60, "annotation_submit"), target.Transition)
}
