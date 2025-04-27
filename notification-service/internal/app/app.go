package app

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/director74/dz8_shop/notification-service/config"
	httpController "github.com/director74/dz8_shop/notification-service/internal/controller/http"
	rabbitmqController "github.com/director74/dz8_shop/notification-service/internal/controller/rabbitmq"
	"github.com/director74/dz8_shop/notification-service/internal/entity"
	"github.com/director74/dz8_shop/notification-service/internal/repo"
	"github.com/director74/dz8_shop/notification-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/database"
	"github.com/director74/dz8_shop/pkg/errors"
	"github.com/director74/dz8_shop/pkg/messaging"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
)

// App представляет приложение
type App struct {
	config     *config.Config
	httpServer *http.Server
	db         *gorm.DB
	router     *gin.Engine
	rabbitMQ   *rabbitmq.RabbitMQ
}

func NewApp(config *config.Config) (*App, error) {
	var db *gorm.DB
	var rmq *rabbitmq.RabbitMQ
	var err error

	// Инициализируем PostgreSQL
	db, err = database.NewPostgresDB(config.Postgres)
	if err != nil {
		return nil, errors.AppendPrefix(err, "не удалось подключиться к базе данных")
	}

	// Автомиграция
	if err := database.AutoMigrateWithCleanup(db, &entity.Notification{}); err != nil {
		return nil, errors.AppendPrefix(err, "не удалось выполнить миграцию")
	}

	// Инициализируем RabbitMQ
	rmq, err = messaging.InitRabbitMQ(config.RabbitMQ)
	if err != nil {
		database.CloseDB(db)
		return nil, errors.AppendPrefix(err, "не удалось подключиться к RabbitMQ")
	}

	// Инициализируем Gin
	router := gin.Default()

	router.Use(errors.RecoveryMiddleware())
	router.Use(errors.ErrorMiddleware())
	router.NoRoute(errors.NotFoundHandler())
	router.NoMethod(errors.MethodNotAllowedHandler())

	httpServer := &http.Server{
		Addr:         ":" + config.HTTP.Port,
		Handler:      router,
		ReadTimeout:  config.HTTP.ReadTimeout,
		WriteTimeout: config.HTTP.WriteTimeout,
	}

	return &App{
		config:     config,
		httpServer: httpServer,
		db:         db,
		router:     router,
		rabbitMQ:   rmq,
	}, nil
}

// Run запускает приложение
func (a *App) Run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// --- Инициализация зависимостей ---
	notificationRepo := repo.NewNotificationRepository(a.db)
	emailSender := usecase.NewDummyEmailSender() // Используем заглушку для email
	notificationUseCase := usecase.NewNotificationUseCase(notificationRepo, emailSender)

	// --- Настройка RabbitMQ ---
	// Инициализируем контроллер консьюмеров
	notificationConsumer := rabbitmqController.NewNotificationConsumer(notificationUseCase, a.rabbitMQ)

	// Определяем имена exchanges
	orderExchangeName := "order_events"
	billingExchangeName := "billing_events"
	sagaExchangeName := "saga_exchange"

	// Настраиваем все очереди и привязки через контроллер
	if err := notificationConsumer.Setup(orderExchangeName, billingExchangeName, sagaExchangeName); err != nil {
		return errors.AppendPrefix(err, "ошибка при настройке notification consumer")
	}

	// Запускаем все консьюмеры через контроллер
	if err := notificationConsumer.StartConsuming(); err != nil {
		return errors.AppendPrefix(err, "ошибка при запуске notification consumers")
	}

	// --- Настройка HTTP ---
	notificationHandler := httpController.NewNotificationHandler(notificationUseCase)
	notificationHandler.RegisterRoutes(a.router)

	// Запускаем HTTP сервер
	go func() {
		log.Printf("HTTP сервер запущен на порту %s", a.config.HTTP.Port)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	// --- Ожидание завершения ---
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-quit:
		log.Println("Получен сигнал завершения, закрываем приложение...")
	case <-ctx.Done():
		log.Println("Контекст завершен, закрываем приложение...")
	}

	return a.Shutdown()
}

// Shutdown корректно завершает работу приложения
func (a *App) Shutdown() error {
	errGroup := errors.NewErrorGroup()

	// Закрываем HTTP сервер
	if a.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := a.httpServer.Shutdown(ctx); err != nil {
			errGroup.AddPrefix(err, "ошибка при закрытии HTTP сервера")
		}
	}

	// Закрываем RabbitMQ
	if a.rabbitMQ != nil {
		a.rabbitMQ.Close()
	}

	// Закрываем БД
	if a.db != nil {
		if err := database.CloseDB(a.db); err != nil {
			errGroup.AddPrefix(err, "ошибка при закрытии соединения с базой данных")
		}
	}

	if errGroup.HasErrors() {
		errors.LogError(errGroup, "Shutdown")
		return errGroup
	}

	log.Println("Приложение успешно завершено")
	return nil
}
