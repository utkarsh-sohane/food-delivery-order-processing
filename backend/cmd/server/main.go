package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/vaurd/food-delivery/internal/config"
	"github.com/vaurd/food-delivery/internal/database"
	"github.com/vaurd/food-delivery/internal/handlers"
	"github.com/vaurd/food-delivery/internal/repository"
	"github.com/vaurd/food-delivery/internal/routes"
	"github.com/vaurd/food-delivery/internal/services"
)

func main() {
	// -----------------------------------------------------------------------
	// Logger — initialise first so every subsequent step can log.
	// -----------------------------------------------------------------------
	logger := buildLogger()
	defer logger.Sync() //nolint:errcheck

	// -----------------------------------------------------------------------
	// Configuration
	// -----------------------------------------------------------------------
	cfg, err := config.Load()
	if err != nil {
		logger.Fatal("failed to load config", zap.Error(err))
	}
	logger.Info("configuration loaded",
		zap.String("serverPort", cfg.ServerPort),
		zap.String("dbHost", cfg.DBHost),
		zap.String("dbName", cfg.DBName),
	)

	// -----------------------------------------------------------------------
	// Database
	// -----------------------------------------------------------------------
	db, err := database.Connect(cfg.DSN(), logger)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	sqlDB, _ := db.DB()
	defer sqlDB.Close()

	// -----------------------------------------------------------------------
	// Dependency wiring (repository → service → handler).
	// -----------------------------------------------------------------------
	orderRepo := repository.NewOrderRepository(db)
	orderSvc := services.NewOrderService(orderRepo, logger)
	eventHandler := handlers.NewEventHandler(orderSvc, logger)
	orderHandler := handlers.NewOrderHandler(orderSvc, logger)

	// -----------------------------------------------------------------------
	// HTTP server
	// -----------------------------------------------------------------------
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	routes.Setup(router, eventHandler, orderHandler, logger)

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.ServerPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start listening in a goroutine so we can listen for shutdown signals below.
	go func() {
		logger.Info("server starting", zap.String("addr", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	// -----------------------------------------------------------------------
	// Graceful shutdown — wait for SIGINT or SIGTERM.
	// -----------------------------------------------------------------------
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("shutdown signal received, draining connections…")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", zap.Error(err))
	} else {
		logger.Info("server shut down cleanly")
	}
}

// buildLogger creates a production-style Zap logger with ISO-8601 timestamps.
func buildLogger() *zap.Logger {
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "timestamp"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig = encoderCfg

	logger, err := logCfg.Build()
	if err != nil {
		// Fallback to a nop logger — should never happen.
		fmt.Fprintf(os.Stderr, "failed to build logger: %v\n", err)
		return zap.NewNop()
	}
	return logger
}
