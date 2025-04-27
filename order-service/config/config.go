package config

import (
	"github.com/director74/dz8_shop/pkg/config"
)

// Config содержит конфигурацию сервиса заказов
type Config struct {
	HTTP     config.HTTPConfig
	Postgres config.PostgresConfig
	RabbitMQ config.RabbitMQConfig
	Services ServicesConfig
	JWT      config.JWTConfig
}

// ServicesConfig содержит настройки внешних сервисов
type ServicesConfig struct {
	BillingURL      string
	NotificationURL string
}

func NewConfig() (*Config, error) {
	// Загружаем общую конфигурацию
	commonConfig := config.LoadCommonConfig("orders", "8080")
	jwtConfig := config.LoadJWTConfig("microservices-auth")
	servicesConfig := config.LoadServicesConfig()

	return &Config{
		HTTP:     commonConfig.HTTP,
		Postgres: commonConfig.Postgres,
		RabbitMQ: commonConfig.RabbitMQ,
		Services: ServicesConfig{
			BillingURL:      servicesConfig.BillingURL,
			NotificationURL: servicesConfig.NotificationURL,
		},
		JWT: *jwtConfig,
	}, nil
}
