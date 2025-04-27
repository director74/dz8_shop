package config

import (
	"github.com/director74/dz8_shop/pkg/config"
)

// Config содержит конфигурацию сервиса биллинга
type Config struct {
	HTTP     config.HTTPConfig
	Postgres config.PostgresConfig
	RabbitMQ config.RabbitMQConfig
	JWT      config.JWTConfig
}

func NewConfig() (*Config, error) {
	// Загружаем общую конфигурацию
	commonConfig := config.LoadCommonConfig("billing", "8081")

	// Загружаем конфигурацию JWT
	jwtConfig := config.LoadJWTConfig("microservices-auth")

	return &Config{
		HTTP:     commonConfig.HTTP,
		Postgres: commonConfig.Postgres,
		RabbitMQ: commonConfig.RabbitMQ,
		JWT:      *jwtConfig,
	}, nil
}
