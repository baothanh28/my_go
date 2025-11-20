package database

import (
	"database/sql"
	"fmt"
	"time"

	"myapp/internal/pkg/config"
	"myapp/internal/pkg/logger"

	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Database wraps gorm.DB
type Database struct {
	*gorm.DB
}

// NewDatabase creates a new database connection
func NewDatabase(cfg *config.Config, log *logger.Logger) (*Database, error) {
	// Build DSN
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	// Configure GORM logger
	gormLog := gormlogger.New(
		&gormLogWriter{logger: log},
		gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Info,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	// Open connection
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLog,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Get underlying SQL DB
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	// Configure connection pool
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetConnMaxLifetime(time.Duration(cfg.Database.ConnMaxLifetime) * time.Second)

	// Test connection
	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	log.Info("Database connection established",
		zap.String("host", cfg.Database.Host),
		zap.Int("port", cfg.Database.Port),
		zap.String("database", cfg.Database.DBName),
	)

	return &Database{DB: db}, nil
}

// gormLogWriter implements gorm logger.Writer interface
type gormLogWriter struct {
	logger *logger.Logger
}

// Printf implements gorm logger.Writer interface
func (w *gormLogWriter) Printf(format string, args ...interface{}) {
	w.logger.Info(fmt.Sprintf(format, args...))
}

// Close closes the database connection
func (d *Database) Close() error {
	sqlDB, err := d.DB.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// Begin starts a transaction
func (d *Database) Begin() *gorm.DB {
	return d.DB.Begin()
}

// RunMigrations runs database migrations (GORM AutoMigrate)
// Deprecated: Use migration package for version-controlled migrations
func (d *Database) RunMigrations(models ...interface{}) error {
	return d.DB.AutoMigrate(models...)
}

// SQLDB returns the underlying *sql.DB for use with migration tools
func (d *Database) SQLDB() (*sql.DB, error) {
	return d.DB.DB()
}
