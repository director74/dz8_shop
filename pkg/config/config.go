package config

import (
	"math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"log"

	"github.com/joho/godotenv"
)

// CommonConfig содержит общую конфигурацию, используемую во всех сервисах
type CommonConfig struct {
	HTTP     HTTPConfig
	Postgres PostgresConfig
	RabbitMQ RabbitMQConfig
}

// HTTPConfig содержит настройки HTTP сервера
type HTTPConfig struct {
	Port         string
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

// PostgresConfig содержит настройки базы данных PostgreSQL
type PostgresConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

// RabbitMQConfig содержит настройки RabbitMQ
type RabbitMQConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

// JWTConfig содержит настройки для JWT
type JWTConfig struct {
	SigningKey     string
	TokenTTL       time.Duration
	TokenIssuer    string
	TokenAudiences []string
}

// ServicesConfig содержит настройки внешних сервисов
type ServicesConfig struct {
	BillingURL      string
	NotificationURL string
}

// LoadCommonConfig загружает общую конфигурацию из переменных окружения
func LoadCommonConfig(serviceName string, port string) *CommonConfig {
	// Загружаем переменные окружения из .env файла, если он существует
	godotenv.Load()

	return &CommonConfig{
		HTTP: HTTPConfig{
			Port:         GetEnv("HTTP_PORT", port),
			ReadTimeout:  GetEnvAsDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			WriteTimeout: GetEnvAsDuration("HTTP_WRITE_TIMEOUT", 10*time.Second),
		},
		Postgres: PostgresConfig{
			Host:     GetEnv("POSTGRES_HOST", "localhost"),
			Port:     GetEnv("POSTGRES_PORT", "5432"),
			User:     GetEnv("POSTGRES_USER", "postgres"),
			Password: GetEnv("POSTGRES_PASSWORD", "postgres"),
			DBName:   GetEnv("POSTGRES_DB", serviceName),
			SSLMode:  GetEnv("POSTGRES_SSLMODE", "disable"),
		},
		RabbitMQ: RabbitMQConfig{
			Host:     GetEnv("RABBITMQ_HOST", "localhost"),
			Port:     GetEnv("RABBITMQ_PORT", "5672"),
			User:     GetEnv("RABBITMQ_USER", "guest"),
			Password: GetEnv("RABBITMQ_PASSWORD", "guest"),
			VHost:    GetEnv("RABBITMQ_VHOST", "/"),
		},
	}
}

// LoadJWTConfig загружает конфигурацию JWT из переменных окружения
func LoadJWTConfig(serviceName string) *JWTConfig {
	signingKey := GetEnv("JWT_SIGNING_KEY", "")
	if signingKey == "" {
		// Генерируем случайный ключ, если не задан
		signingKey = GenerateRandomKey(32)
		log.Println("ВНИМАНИЕ: JWT_SIGNING_KEY не задан! Сгенерирован случайный ключ. Для работы JWT между сервисами необходимо указать одинаковый JWT_SIGNING_KEY во всех сервисах.")
	}

	return &JWTConfig{
		SigningKey:     signingKey,
		TokenTTL:       GetEnvAsDuration("JWT_TOKEN_TTL", 24*time.Hour),
		TokenIssuer:    GetEnv("JWT_TOKEN_ISSUER", serviceName),
		TokenAudiences: strings.Split(GetEnv("JWT_TOKEN_AUDIENCES", "microservices"), ","),
	}
}

// LoadServicesConfig загружает конфигурацию внешних сервисов из переменных окружения
func LoadServicesConfig() *ServicesConfig {
	return &ServicesConfig{
		BillingURL:      GetEnv("BILLING_SERVICE_URL", "http://localhost:8081"),
		NotificationURL: GetEnv("NOTIFICATION_SERVICE_URL", "http://localhost:8082"),
	}
}

// GenerateRandomKey генерирует случайный ключ заданной длины
func GenerateRandomKey(length int) string {
	// Инициализируем генератор случайных чисел
	rand.Seed(time.Now().UnixNano())

	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

func GetEnv(key, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func GetEnvAsInt(key string, defaultValue int) int {
	valueStr := GetEnv(key, "")
	if value, err := strconv.Atoi(valueStr); err == nil {
		return value
	}
	return defaultValue
}

func GetEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := GetEnv(key, "")
	if value, err := time.ParseDuration(valueStr); err == nil {
		return value
	}
	return defaultValue
}
