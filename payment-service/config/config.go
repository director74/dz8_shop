package config

import (
	"github.com/director74/dz8_shop/pkg/config"
)

// Config содержит конфигурацию платежного сервиса
type Config struct {
	HTTP     config.HTTPConfig
	Postgres config.PostgresConfig
	RabbitMQ config.RabbitMQConfig
	JWT      config.JWTConfig
	Internal InternalAPIConfig
}

// InternalAPIConfig конфигурация для внутреннего API
type InternalAPIConfig struct {
	TrustedNetworks []string
	APIKeyEnvName   string
	DefaultAPIKey   string
	HeaderName      string
}

// NewConfig создает новую конфигурацию платежного сервиса
func NewConfig() (*Config, error) {
	// Загружаем общую конфигурацию
	commonConfig := config.LoadCommonConfig("payments", "8083")

	// Загружаем конфигурацию JWT
	jwtConfig := config.LoadJWTConfig("microservices-auth")

	// Загружаем конфигурацию для внутреннего API
	internalConfig := loadInternalAPIConfig()

	return &Config{
		HTTP:     commonConfig.HTTP,
		Postgres: commonConfig.Postgres,
		RabbitMQ: commonConfig.RabbitMQ,
		JWT:      *jwtConfig,
		Internal: internalConfig,
	}, nil
}

// loadInternalAPIConfig загружает конфигурацию для внутреннего API
func loadInternalAPIConfig() InternalAPIConfig {
	// Здесь можно добавить загрузку из файла конфигурации или переменных окружения
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
