package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

// Config содержит настройки подключения к RabbitMQ
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	VHost    string
}

// RabbitMQ представляет клиент для работы с RabbitMQ
type RabbitMQ struct {
	config     Config
	connection *amqp.Connection
	channel    *amqp.Channel
}

func NewRabbitMQ(cfg Config) (*RabbitMQ, error) {
	rmq := &RabbitMQ{
		config: cfg,
	}

	err := rmq.connect()
	if err != nil {
		return nil, err
	}

	return rmq, nil
}

// connect устанавливает соединение с RabbitMQ
func (r *RabbitMQ) connect() error {
	connStr := fmt.Sprintf("amqp://%s:%s@%s:%s/%s",
		r.config.User, r.config.Password, r.config.Host, r.config.Port, r.config.VHost)

	conn, err := amqp.Dial(connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}
	r.connection = conn

	ch, err := conn.Channel()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to open channel: %w", err)
	}
	r.channel = ch

	return nil
}

// reconnect пытается восстановить соединение с RabbitMQ
func (r *RabbitMQ) reconnect() error {
	if r.connection != nil && !r.connection.IsClosed() {
		return nil
	}

	log.Println("Попытка переподключения к RabbitMQ...")
	return r.connect()
}

// Close закрывает соединение с RabbitMQ
func (r *RabbitMQ) Close() error {
	var err error
	if r.channel != nil {
		if err = r.channel.Close(); err != nil {
			return fmt.Errorf("ошибка при закрытии канала: %w", err)
		}
	}
	if r.connection != nil {
		if err = r.connection.Close(); err != nil {
			return fmt.Errorf("ошибка при закрытии соединения: %w", err)
		}
	}
	return nil
}

// DeclareExchange объявляет exchange
func (r *RabbitMQ) DeclareExchange(name string, kind string) error {
	if err := r.reconnect(); err != nil {
		return fmt.Errorf("ошибка переподключения перед объявлением exchange: %w", err)
	}

	return r.channel.ExchangeDeclare(
		name,  // name
		kind,  // type
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
}

// DeclareQueue объявляет очередь
func (r *RabbitMQ) DeclareQueue(name string) error {
	if err := r.reconnect(); err != nil {
		return fmt.Errorf("ошибка переподключения перед объявлением очереди: %w", err)
	}

	_, err := r.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
	return err
}

// DeclareQueueWithReturn объявляет очередь и возвращает информацию о ней
func (r *RabbitMQ) DeclareQueueWithReturn(name string) (amqp.Queue, error) {
	if err := r.reconnect(); err != nil {
		return amqp.Queue{}, fmt.Errorf("ошибка переподключения перед объявлением очереди: %w", err)
	}

	return r.channel.QueueDeclare(
		name,  // name
		true,  // durable
		false, // delete when unused
		false, // exclusive
		false, // no-wait
		nil,   // arguments
	)
}

// BindQueue привязывает очередь к exchange
func (r *RabbitMQ) BindQueue(queueName, exchangeName, routingKey string) error {
	if err := r.reconnect(); err != nil {
		return fmt.Errorf("ошибка переподключения перед привязкой очереди: %w", err)
	}

	return r.channel.QueueBind(
		queueName,    // queue name
		routingKey,   // routing key
		exchangeName, // exchange
		false,        // no-wait
		nil,          // arguments
	)
}

// PublishMessage публикует сообщение в RabbitMQ
func (r *RabbitMQ) PublishMessage(exchange, routingKey string, message interface{}) error {
	if err := r.reconnect(); err != nil {
		return fmt.Errorf("ошибка переподключения перед публикацией сообщения: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	body, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	return r.channel.PublishWithContext(
		ctx,
		exchange,   // exchange
		routingKey, // routing key
		false,      // mandatory
		false,      // immediate
		amqp.Publishing{
			ContentType:  "application/json",
			DeliveryMode: amqp.Persistent,
			Body:         body,
		},
	)
}

// PublishMessageWithRetry публикует сообщение с повторными попытками
func (r *RabbitMQ) PublishMessageWithRetry(exchange, routingKey string, message interface{}, retries int) error {
	var err error
	for i := 0; i <= retries; i++ {
		if err = r.PublishMessage(exchange, routingKey, message); err == nil {
			return nil
		}

		log.Printf("Ошибка публикации сообщения (попытка %d/%d): %v", i+1, retries+1, err)

		if i < retries {
			backoff := time.Duration(i+1) * time.Second
			log.Printf("Повторная попытка через %v...", backoff)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("не удалось опубликовать сообщение после %d попыток: %w", retries+1, err)
}

// ConsumeMessages начинает обработку сообщений из очереди с обработчиком
func (r *RabbitMQ) ConsumeMessages(queueName, consumerName string, handler func([]byte) error) error {
	if err := r.reconnect(); err != nil {
		return fmt.Errorf("ошибка переподключения перед обработкой сообщений: %w", err)
	}

	// Добавляем уникальный идентификатор к имени консьюмера, если он ещё не содержит временную метку
	if !containsTimestamp(consumerName) {
		consumerName = fmt.Sprintf("%s-%d", consumerName, time.Now().UnixNano())
	}

	msgs, err := r.channel.Consume(
		queueName,    // queue
		consumerName, // consumer
		false,        // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // args
	)

	if err != nil {
		return fmt.Errorf("ошибка при начале обработки сообщений: %w", err)
	}

	go r.HandleMessages(msgs, handler)

	return nil
}

// containsTimestamp проверяет, содержит ли строка числовой суффикс, похожий на временную метку
func containsTimestamp(s string) bool {
	// Простая эвристика для проверки: строка должна заканчиваться на минимум 10 цифр подряд
	// (примерно длина Unix timestamp)
	var consecutiveDigits int
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] >= '0' && s[i] <= '9' {
			consecutiveDigits++
			if consecutiveDigits >= 10 {
				return true
			}
		} else {
			consecutiveDigits = 0
		}
	}
	return false
}

func (r *RabbitMQ) HandleMessages(msgs <-chan amqp.Delivery, handler func([]byte) error) {
	for msg := range msgs {
		err := handler(msg.Body)
		if err != nil {
			log.Printf("Error handling message: %v", err)
			msg.Nack(false, true) // Сообщение не обработано и возвращается в очередь
		} else {
			msg.Ack(false) // Подтверждаем обработку сообщения
		}
	}
}
