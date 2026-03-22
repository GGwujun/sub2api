package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterWeComRoutes registers WeCom public API routes.
func RegisterWeComRoutes(v1 *gin.RouterGroup, h *handler.Handlers, adminAuth middleware2.AdminAuthMiddleware) {
	channel := v1.Group("/channel")
	channel.Use(gin.HandlerFunc(adminAuth))
	{
		channel.POST("/content/generate", h.WeCom.GenerateChannelContent)
	}
}
