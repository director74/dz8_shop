package database

import (
	"fmt"

	"github.com/director74/dz8_shop/pkg/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// NewPostgresDB создает новое подключение к PostgreSQL с общими параметрами
func NewPostgresDB(cfg config.PostgresConfig) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.DBName, cfg.SSLMode)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database connection: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}

// AutoMigrateWithCleanup выполняет автоматическую миграцию моделей с корректной обработкой ошибок и освобождением ресурсов
func AutoMigrateWithCleanup(db *gorm.DB, models ...interface{}) error {
	if err := db.AutoMigrate(models...); err != nil {
		sqlDB, sqlErr := db.DB()
		if sqlErr == nil && sqlDB != nil {
			sqlDB.Close()
		}
		return fmt.Errorf("не удалось выполнить миграцию: %w", err)
	}
	return nil
}

// CloseDB закрывает соединение с базой данных с корректной обработкой ошибок
func CloseDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}

	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("ошибка при получении SQL DB: %w", err)
	}

	if sqlDB != nil {
		if err := sqlDB.Close(); err != nil {
			return fmt.Errorf("ошибка при закрытии соединения с базой данных: %w", err)
		}
	}

	return nil
}
