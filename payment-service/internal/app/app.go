package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/director74/dz8_shop/payment-service/config"
	httpController "github.com/director74/dz8_shop/payment-service/internal/controller/http"
	rmqController "github.com/director74/dz8_shop/payment-service/internal/controller/rabbitmq"
	"github.com/director74/dz8_shop/payment-service/internal/entity"
	"github.com/director74/dz8_shop/payment-service/internal/repo"
	"github.com/director74/dz8_shop/payment-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/auth"
	"github.com/director74/dz8_shop/pkg/database"
	"github.com/director74/dz8_shop/pkg/errors"
	"github.com/director74/dz8_shop/pkg/messaging"

	// nolint:typecheck
	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// App представляет основное приложение платежного сервиса
// Внутренние API эндпоинты (/internal/*) предназначены только для взаимодействия между микросервисами
type App struct {
	config   *config.Config
	db       *gorm.DB
	rabbitMQ messaging.MessageBroker
	router   *gin.Engine
	server   *http.Server
}

// NewApp создает новое приложение с указанной конфигурацией
func NewApp(cfg *config.Config) (*App, error) {
	var db *gorm.DB
	var rmq messaging.MessageBroker
	var err error

	// Инициализируем подключение к PostgreSQL
	db, err = database.NewPostgresDB(cfg.Postgres)
	if err != nil {
		return nil, errors.AppendPrefix(err, "не удалось подключиться к базе данных")
	}

	// Автомиграция моделей
	if err := database.AutoMigrateWithCleanup(db, &entity.Payment{}, &entity.PaymentMethod{}); err != nil {
		return nil, errors.AppendPrefix(err, "не удалось выполнить миграцию")
	}

	// Инициализируем подключение к RabbitMQ
	rmq, err = messaging.InitRabbitMQ(cfg.RabbitMQ)
	if err != nil {
		database.CloseDB(db)
		return nil, errors.AppendPrefix(err, "не удалось подключиться к RabbitMQ")
	}

	// Настраиваем exchanges и очереди в RabbitMQ
	exchanges := map[string]string{
		"payment_events": "topic",
		"order_events":   "topic",
	}
	queues := map[string]map[string]string{
		"order_payment_queue": {
			"order_events": "order.created",
		},
	}

	if err := messaging.SetupExchangesAndQueues(rmq, exchanges, queues); err != nil {
		database.CloseDB(db)
		rmq.Close()
		return nil, errors.AppendPrefix(err, "ошибка при настройке RabbitMQ")
	}

	// Инициализируем JWT менеджер
	jwtConfig := &auth.Config{
		SigningKey:     cfg.JWT.SigningKey,
		TokenTTL:       cfg.JWT.TokenTTL,
		TokenIssuer:    cfg.JWT.TokenIssuer,
		TokenAudiences: cfg.JWT.TokenAudiences,
	}
	jwtManager := auth.NewJWTManager(jwtConfig)

	// Создаем middleware для авторизации
	authMiddleware := auth.NewAuthMiddleware(jwtManager)

	// Создание роутера
	router := gin.Default()

	// Создание репозитория платежей
	paymentRepo := repo.NewPaymentRepository(db)

	// Создание use case платежей
	paymentUseCase := usecase.NewPaymentUseCase(paymentRepo, rmq, "payment_events")

	// Создание обработчика HTTP запросов
	paymentHandler := httpController.NewPaymentHandler(paymentUseCase, cfg)

	// Проверяем, что RabbitMQ имеет правильный тип
	rawRMQ, ok := rmq.(*rabbitmq.RabbitMQ)
	if !ok {
		database.CloseDB(db)
		rmq.Close()
		return nil, fmt.Errorf("неожиданный тип для RabbitMQ: %T", rmq)
	}

	// Создание обработчика сообщений RabbitMQ
	paymentConsumer := rmqController.NewPaymentConsumer(paymentUseCase, rawRMQ)

	// Создание обработчика сообщений саги
	sagaConsumer := rmqController.NewSagaConsumer(paymentUseCase, rawRMQ)

	// Регистрация маршрутов
	paymentHandler.RegisterRoutes(router, authMiddleware.AuthRequired())

	// Настройка обработки сообщений RabbitMQ
	if err := paymentConsumer.Setup(); err != nil {
		database.CloseDB(db)
		rmq.Close()
		return nil, errors.AppendPrefix(err, "ошибка настройки обработчика сообщений")
	}

	// Настройка обработки сообщений саги
	if err := sagaConsumer.Setup(); err != nil {
		database.CloseDB(db)
		rmq.Close()
		return nil, errors.AppendPrefix(err, "ошибка настройки обработчика сообщений саги")
	}

	// Настройка HTTP сервера
	server := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.HTTP.Port),
		Handler: router,
	}

	return &App{
		config:   cfg,
		db:       db,
		rabbitMQ: rmq,
		router:   router,
		server:   server,
	}, nil
}

// Run запускает приложение
func (a *App) Run() error {
	// Запуск HTTP сервера
	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("ошибка запуска HTTP сервера: %v", err)
		}
	}()

	log.Printf("Платежный сервис запущен на порту %s", a.config.HTTP.Port)

	// Ожидание сигнала для грациозного завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Завершение работы платежного сервиса...")

	// Завершение HTTP сервера
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf("ошибка остановки HTTP сервера: %v", err)
	}

	// Закрытие соединения с RabbitMQ
	if err := a.rabbitMQ.Close(); err != nil {
		log.Printf("ошибка закрытия соединения с RabbitMQ: %v", err)
	}

	log.Println("Платежный сервис остановлен")
	return nil
}

// Healthcheck проверяет работоспособность сервиса
func (a *App) Healthcheck() error {
	// Проверка соединения с базой данных
	sql, err := a.db.DB()
	if err != nil {
		return err
	}

	if err := sql.Ping(); err != nil {
		return err
	}

	return nil
}
