package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/worryyy/k3s-platform/platform/server/internal/release"
)

type RouterDependencies struct {
	Releases *release.Service
	Store    interface {
		Ping(ctx context.Context) error
	}
}

func NewRouter(deps RouterDependencies) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())

	serviceHandler := ServiceHandler{releases: deps.Releases}
	releaseHandler := ReleaseHandler{releases: deps.Releases}

	registerHealthRoutes(router, deps)

	api := router.Group("/api")
	registerServiceRoutes(api, serviceHandler)
	registerReleaseRoutes(api, releaseHandler)

	return router
}

func registerServiceRoutes(api *gin.RouterGroup, handler ServiceHandler) {
	api.GET("/services", handler.List)
	api.GET("/services/:name", handler.Get)
	api.GET("/services/:name/status", handler.Status)
}

func registerReleaseRoutes(api *gin.RouterGroup, handler ReleaseHandler) {
	api.POST("/releases", handler.Create)
	api.GET("/releases/:id", handler.Get)
	api.GET("/releases/:id/events", handler.Events)
	api.GET("/services/:name/releases", handler.ListByService)
}

func registerHealthRoutes(router *gin.Engine, deps RouterDependencies) {
	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, healthResponse{Status: "ok", Time: time.Now().UTC()})
	})
	router.GET("/readyz", func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()
		if deps.Store != nil {
			if err := deps.Store.Ping(ctx); err != nil {
				c.JSON(http.StatusServiceUnavailable, readinessResponse{Status: "not_ready"})
				return
			}
		}
		c.JSON(http.StatusOK, readinessResponse{Status: "ready"})
	})
}
