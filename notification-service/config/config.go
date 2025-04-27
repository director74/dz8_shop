package config

import (
	"github.com/director74/dz8_shop/pkg/config"
)

// Config содержит конфигурацию сервиса уведомлений
type Config struct {
	HTTP     config.HTTPConfig
	Postgres config.PostgresConfig
	RabbitMQ config.RabbitMQConfig
	Mail     MailConfig
}

// MailConfig содержит настройки для отправки почты
type MailConfig struct {
	SMTPHost     string
	SMTPPort     string
	SMTPUser     string
	SMTPPassword string
	FromEmail    string
}

// LoadMailConfig загружает конфигурацию для отправки почты
func LoadMailConfig() MailConfig {
	return MailConfig{
		SMTPHost:     config.GetEnv("SMTP_HOST", "localhost"),
		SMTPPort:     config.GetEnv("SMTP_PORT", "1025"),
		SMTPUser:     config.GetEnv("SMTP_USER", ""),
		SMTPPassword: config.GetEnv("SMTP_PASSWORD", ""),
		FromEmail:    config.GetEnv("FROM_EMAIL", "notification@example.com"),
	}
}

func NewConfig() (*Config, error) {
	// Загружаем общую конфигурацию
	commonConfig := config.LoadCommonConfig("notifications", "8082")
	mailConfig := LoadMailConfig()

	return &Config{
		HTTP:     commonConfig.HTTP,
		Postgres: commonConfig.Postgres,
		RabbitMQ: commonConfig.RabbitMQ,
		Mail:     mailConfig,
	}, nil
}
