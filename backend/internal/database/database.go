package database

import (
	"fmt"
	"time"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/vaurd/food-delivery/internal/models"
)

// Connect opens a connection to PostgreSQL using GORM, runs AutoMigrate to ensure
// the schema is up to date, and returns the *gorm.DB instance.
func Connect(dsn string, logger *zap.Logger) (*gorm.DB, error) {
	// Use the silent GORM logger in production; real query logging goes through Zap.
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open db connection: %w", err)
	}

	// Configure the underlying *sql.DB connection pool.
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying sql.DB: %w", err)
	}
	sqlDB.SetMaxOpenConns(25)
	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	// Ensure the schema exists. AutoMigrate is safe to call on every startup.
	if err := autoMigrate(db); err != nil {
		return nil, fmt.Errorf("auto migrate: %w", err)
	}

	// Create performance indexes (idempotent – IF NOT EXISTS).
	if err := createIndexes(db); err != nil {
		return nil, fmt.Errorf("create indexes: %w", err)
	}

	logger.Info("database connected and schema migrated")
	return db, nil
}

func autoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(&models.Order{})
}

func createIndexes(db *gorm.DB) error {
	statements := []string{
		`CREATE INDEX IF NOT EXISTS idx_orders_status     ON orders(status)`,
		`CREATE INDEX IF NOT EXISTS idx_orders_updated_at ON orders(updated_at DESC)`,
	}
	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("exec '%s': %w", stmt, err)
		}
	}
	return nil
}
