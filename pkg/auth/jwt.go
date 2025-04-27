package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenClaims содержит данные пользователя и стандартные JWT claims
type TokenClaims struct {
	UserID   uint   `json:"user_id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	jwt.RegisteredClaims
}

// Config содержит настройки для JWT токенов
type Config struct {
	SigningKey     string
	TokenTTL       time.Duration
	SigningMethod  jwt.SigningMethod
	TokenIssuer    string
	TokenAudiences []string
}

func NewConfig(signingKey string) *Config {
	return &Config{
		SigningKey:     signingKey,
		TokenTTL:       24 * time.Hour,
		SigningMethod:  jwt.SigningMethodHS256,
		TokenIssuer:    "auth-service",
		TokenAudiences: []string{"microservices"},
	}
}

// JWTManager управляет JWT токенами
type JWTManager struct {
	config *Config
}

func NewJWTManager(config *Config) *JWTManager {
	return &JWTManager{
		config: config,
	}
}

// GenerateToken создаёт JWT токен с данными пользователя и временем истечения,
// установленным в конфигурации
func (m *JWTManager) GenerateToken(userID uint, username, email string) (string, error) {
	now := time.Now()
	claims := TokenClaims{
		UserID:   userID,
		Username: username,
		Email:    email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(m.config.TokenTTL)),
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			Issuer:    m.config.TokenIssuer,
			Audience:  m.config.TokenAudiences,
		},
	}

	token := jwt.NewWithClaims(m.config.SigningMethod, claims)
	return token.SignedString([]byte(m.config.SigningKey))
}

// ParseToken проверяет валидность JWT токена и извлекает из него данные
func (m *JWTManager) ParseToken(tokenString string) (*TokenClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("неожиданный метод подписи: %v", token.Header["alg"])
		}
		return []byte(m.config.SigningKey), nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*TokenClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("недействительный токен")
}
