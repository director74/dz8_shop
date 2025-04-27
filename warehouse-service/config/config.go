package config

import (
	"time"

	"github.com/director74/dz8_shop/pkg/config"
)

// Config содержит конфигурацию сервиса склада
type Config struct {
	HTTP      config.HTTPConfig
	Postgres  config.PostgresConfig
	RabbitMQ  config.RabbitMQConfig
	JWT       config.JWTConfig
	Warehouse WarehouseConfig
	Internal  InternalAPIConfig
}

// WarehouseConfig содержит специфичные настройки для сервиса склада
type WarehouseConfig struct {
	ReservationTTL time.Duration
}

// InternalAPIConfig конфигурация для внутреннего API
type InternalAPIConfig struct {
	TrustedNetworks []string
	APIKeyEnvName   string
	DefaultAPIKey   string
	HeaderName      string
}

// NewConfig создает новую конфигурацию сервиса склада
func NewConfig() (*Config, error) {
	// Загружаем общую конфигурацию
	commonConfig := config.LoadCommonConfig("warehouse", "8084")

	// Загружаем конфигурацию JWT
	jwtConfig := config.LoadJWTConfig("microservices-auth")

	// Настройки склада
	warehouseConfig := loadWarehouseConfig()

	// Настройки для внутреннего API
	internalConfig := loadInternalAPIConfig()

	return &Config{
		HTTP:      commonConfig.HTTP,
		Postgres:  commonConfig.Postgres,
		RabbitMQ:  commonConfig.RabbitMQ,
		JWT:       *jwtConfig,
		Warehouse: warehouseConfig,
		Internal:  internalConfig,
	}, nil
}

// loadWarehouseConfig загружает специфичные настройки склада
func loadWarehouseConfig() WarehouseConfig {
	return WarehouseConfig{
		ReservationTTL: config.GetEnvAsDuration("WAREHOUSE_RESERVATION_TTL", 30*time.Minute),
	}
}

// loadInternalAPIConfig загружает конфигурацию для внутреннего API
func loadInternalAPIConfig() InternalAPIConfig {
	return InternalAPIConfig{
		TrustedNetworks: []string{
			"10.0.0.0/8",     // Внутренняя сеть Kubernetes
			"172.16.0.0/12",  // Docker сеть по умолчанию
			"192.168.0.0/16", // Локальная сеть
			"127.0.0.0/8",    // Локальный хост
		},
		APIKeyEnvName: "INTERNAL_API_KEY",
		DefaultAPIKey: "internal-api-key-for-development",
		HeaderName:    "X-Internal-API-Key",
	}
}
