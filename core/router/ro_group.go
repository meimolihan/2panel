package router

import (
	v2 "github.com/2Panel-dev/2Panel/core/app/api/v2"
	"github.com/2Panel-dev/2Panel/core/middleware"
	"github.com/gin-gonic/gin"
)

type GroupRouter struct {
}

func (a *GroupRouter) InitRouter(Router *gin.RouterGroup) {
	groupRouter := Router.Group("groups").
		Use(middleware.SessionAuth()).
		Use(middleware.PasswordExpired())

	baseApi := v2.ApiGroupApp.BaseApi
	{
		groupRouter.POST("", baseApi.CreateGroup)
		groupRouter.POST("/del", baseApi.DeleteGroup)
		groupRouter.POST("/update", baseApi.UpdateGroup)
		groupRouter.POST("/search", baseApi.ListGroup)
	}
}
