package middleware

import (
	"net"
	"os"

	"github.com/gin-gonic/gin"
)

// InternalAPIConfig конфигурация для внутреннего API
type InternalAPIConfig struct {
	// TrustedNetworks список доверенных IP-адресов или CIDR диапазонов
	TrustedNetworks []string
	// APIKeyEnvName имя переменной окружения, где хранится ключ API
	APIKeyEnvName string
	// DefaultAPIKey ключ по умолчанию, если не задан через переменные окружения
	DefaultAPIKey string
	// HeaderName имя заголовка для передачи ключа API
	HeaderName string
}

// NewInternalAPIConfig создает конфигурацию по умолчанию
func NewInternalAPIConfig() *InternalAPIConfig {
	return &InternalAPIConfig{
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

// InternalAuthMiddleware middleware для защиты доступа к внутренним API
type InternalAuthMiddleware struct {
	config *InternalAPIConfig
	apiKey string
}

// NewInternalAuthMiddleware создает новый middleware для защиты внутренних API
func NewInternalAuthMiddleware(config *InternalAPIConfig) *InternalAuthMiddleware {
	if config == nil {
		config = NewInternalAPIConfig()
	}

	// Получаем ключ API из переменной окружения или используем значение по умолчанию
	apiKey := os.Getenv(config.APIKeyEnvName)
	if apiKey == "" {
		apiKey = config.DefaultAPIKey
	}

	return &InternalAuthMiddleware{
		config: config,
		apiKey: apiKey,
	}
}

// Required middleware требует авторизации для доступа к внутренним API
// Проверяет либо наличие корректного API ключа, либо что запрос идет из доверенной сети
func (m *InternalAuthMiddleware) Required() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Проверка API ключа в заголовке
		headerKey := c.GetHeader(m.config.HeaderName)
		if headerKey == m.apiKey {
			c.Next()
			return
		}

		// Если ключ не верный, проверяем IP-адрес
		clientIP := c.ClientIP()

		// Проверяем, что IP адрес входит в список доверенных сетей
		if isIPTrusted(clientIP, m.config.TrustedNetworks) {
			c.Next()
			return
		}

		// Если ни ключ, ни IP не прошли проверку, запрещаем доступ
		c.AbortWithStatusJSON(403, gin.H{
			"error": "доступ запрещен, этот API доступен только для внутренних сервисов",
		})
	}
}

// isIPTrusted проверяет, входит ли IP-адрес в список доверенных сетей
func isIPTrusted(ipStr string, trustedNetworks []string) bool {
	// Обработка IPv4 и IPv6 адресов
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}

	// Проверяем, входит ли IP в один из доверенных диапазонов CIDR
	for _, network := range trustedNetworks {
		_, ipNet, err := net.ParseCIDR(network)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}

	return false
}
