package config

import (
	"github.com/director74/dz8_shop/pkg/config"
)

// Config содержит конфигурацию сервиса доставки
type Config struct {
	HTTP     config.HTTPConfig
	Postgres config.PostgresConfig
	RabbitMQ config.RabbitMQConfig
	JWT      config.JWTConfig
	Delivery DeliveryConfig
	Internal InternalAPIConfig
}

// DeliveryConfig содержит специфичные настройки для сервиса доставки
type DeliveryConfig struct {
	SlotDuration       string `mapstructure:"slot_duration"`
	DefaultSlotCapacity int    `mapstructure:"default_slot_capacity"`
}

// InternalAPIConfig конфигурация для внутреннего API
type InternalAPIConfig struct {
	TrustedNetworks []string
	APIKeyEnvName   string
	DefaultAPIKey   string
	HeaderName      string
}

// NewConfig создает новую конфигурацию сервиса доставки
func NewConfig() (*Config, error) {
	// Загружаем общую конфигурацию
	commonConfig := config.LoadCommonConfig("delivery", "8084")

	// Загружаем конфигурацию JWT
	jwtConfig := config.LoadJWTConfig("microservices-auth")

	// Настройки доставки
	deliveryConfig := loadDeliveryConfig()

	// Настройки для внутреннего API
	internalConfig := loadInternalAPIConfig()

	return &Config{
		HTTP:     commonConfig.HTTP,
		Postgres: commonConfig.Postgres,
		RabbitMQ: commonConfig.RabbitMQ,
		JWT:      *jwtConfig,
		Delivery: deliveryConfig,
		Internal: internalConfig,
	}, nil
}

// loadDeliveryConfig загружает специфичные настройки доставки
func loadDeliveryConfig() DeliveryConfig {
	return DeliveryConfig{
		SlotDuration:       config.GetEnv("DELIVERY_SLOT_DURATION", "1h"), // По умолчанию один час
		DefaultSlotCapacity: config.GetEnvAsInt("DELIVERY_SLOT_CAPACITY", 5), // По умолчанию 5 курьеров на слот
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