package messaging

import (
	"log"

	"github.com/director74/dz8_shop/pkg/config"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
)

// MessagePublisher интерфейс для публикации сообщений
type MessagePublisher interface {
	PublishMessage(exchange, routingKey string, message interface{}) error
	PublishMessageWithRetry(exchange, routingKey string, message interface{}, retries int) error
}

// MessageConsumer интерфейс для получения сообщений
type MessageConsumer interface {
	DeclareQueue(name string) error
	BindQueue(queueName, exchangeName, routingKey string) error
	ConsumeMessages(queueName, consumerName string, handler func([]byte) error) error
}

// MessageBroker объединяет функциональность публикации и обработки сообщений
type MessageBroker interface {
	MessagePublisher
	MessageConsumer
	DeclareExchange(name string, kind string) error
	Close() error
}

// InitRabbitMQ инициализирует подключение к RabbitMQ с общими параметрами
func InitRabbitMQ(cfg config.RabbitMQConfig) (*rabbitmq.RabbitMQ, error) {
	rmqCfg := rabbitmq.Config{
		Host:     cfg.Host,
		Port:     cfg.Port,
		User:     cfg.User,
		Password: cfg.Password,
		VHost:    cfg.VHost,
	}

	rmq, err := rabbitmq.NewRabbitMQ(rmqCfg)
	if err != nil {
		return nil, err
	}

	return rmq, nil
}

// PublishWithLogging публикует сообщение с логированием успеха/ошибки
func PublishWithLogging(publisher MessagePublisher, exchange, routingKey string, message interface{}) error {
	err := publisher.PublishMessage(exchange, routingKey, message)
	if err != nil {
		log.Printf("Ошибка при публикации сообщения в %s с ключом %s: %v", exchange, routingKey, err)
		return err
	}

	log.Printf("Сообщение успешно опубликовано в %s с ключом %s", exchange, routingKey)
	return nil
}

// PublishWithRetryAndLogging публикует сообщение с повторными попытками и логированием
func PublishWithRetryAndLogging(publisher MessagePublisher, exchange, routingKey string, message interface{}, retries int) error {
	err := publisher.PublishMessageWithRetry(exchange, routingKey, message, retries)
	if err != nil {
		log.Printf("Ошибка при публикации сообщения в %s с ключом %s после %d попыток: %v",
			exchange, routingKey, retries+1, err)
		return err
	}

	log.Printf("Сообщение успешно опубликовано в %s с ключом %s после повторных попыток", exchange, routingKey)
	return nil
}

// SetupExchangesAndQueues настраивает exchanges и очереди для сервиса
func SetupExchangesAndQueues(broker MessageBroker, exchanges map[string]string, queues map[string]map[string]string) error {
	// Настраиваем exchanges
	for name, kind := range exchanges {
		if err := broker.DeclareExchange(name, kind); err != nil {
			return err
		}
	}

	// Настраиваем очереди и их привязки
	for queueName, bindings := range queues {
		if err := broker.DeclareQueue(queueName); err != nil {
			return err
		}

		for exchangeName, routingKey := range bindings {
			if err := broker.BindQueue(queueName, exchangeName, routingKey); err != nil {
				return err
			}
		}
	}

	return nil
}
