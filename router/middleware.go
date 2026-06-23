//go:build !cloudflare

package router

import (
	"github.com/gin-gonic/gin"
	//"gitlab.com/protiva-cor/gateway/router/middleware/jwt"
	"github.com/apito-io/engine/models"
)

func InitMiddleware(router *gin.Engine, cfg *models.Config) {
	router.Use(gin.Logger())
	router.Use(gin.Recovery())
}
