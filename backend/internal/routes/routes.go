package routes

import (
	"github.com/gin-gonic/gin"

	"github.com/vaurd/food-delivery/internal/handlers"
	"github.com/vaurd/food-delivery/internal/middleware"
	"go.uber.org/zap"
)

// Setup registers all application routes on the provided Gin engine.
// Dependencies are passed in directly, keeping this function pure and testable.
func Setup(
	router *gin.Engine,
	eventHandler *handlers.EventHandler,
	orderHandler *handlers.OrderHandler,
	logger *zap.Logger,
) {
	// Global middleware.
	router.Use(middleware.ZapLogger(logger))
	router.Use(gin.Recovery())

	// Health check — useful for docker-compose health checks and load-balancer probes.
	router.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// Event ingestion endpoint.
	router.POST("/events", eventHandler.Handle)

	// Order query endpoints.
	router.GET("/orders", orderHandler.ListOrders)
	router.GET("/orders/:id", orderHandler.GetOrder)
}
