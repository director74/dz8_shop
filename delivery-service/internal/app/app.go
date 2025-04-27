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

	"github.com/director74/dz8_shop/delivery-service/config"
	httpController "github.com/director74/dz8_shop/delivery-service/internal/controller/http"
	"github.com/director74/dz8_shop/delivery-service/internal/controller/rabbitmq"
	"github.com/director74/dz8_shop/delivery-service/internal/repo"
	"github.com/director74/dz8_shop/delivery-service/internal/usecase"
	pkgRabbitMQ "github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// App представляет приложение службы доставки
type App struct {
	httpServer      *http.Server
	deliveryUseCase *usecase.DeliveryUseCase
	deliveryRepo    *repo.DeliveryRepo
	config          *config.Config
	db              *gorm.DB
	router          *gin.Engine
	sagaConsumer    *rabbitmq.SagaConsumer
	rabbitMQ        *pkgRabbitMQ.RabbitMQ
}

// NewApp создает новый экземпляр приложения
func NewApp(config *config.Config) (*App, error) {
	// Инициализируем подключение к базе данных
	db, err := initDB(config)
	if err != nil {
		return nil, err
	}

	// Инициализируем подключение к RabbitMQ
	rabbitMQ, err := initRabbitMQ(config)
	if err != nil {
		return nil, err
	}

	// Инициализируем репозиторий
	deliveryRepo := repo.NewDeliveryRepo(db)

	// Инициализируем use case
	deliveryUseCase := usecase.NewDeliveryUseCase(deliveryRepo, rabbitMQ, "saga_exchange")

	// Инициализируем обработчик HTTP запросов
	router := gin.Default()
	deliveryHandler := httpController.NewDeliveryHandler(deliveryUseCase)
	deliveryHandler.RegisterRoutes(router)

	// Инициализируем обработчик сообщений саги
	sagaConsumer := rabbitmq.NewSagaConsumer(deliveryUseCase, rabbitMQ)

	return &App{
		httpServer: &http.Server{
			Addr:         ":" + config.HTTP.Port,
			Handler:      router,
			ReadTimeout:  config.HTTP.ReadTimeout,
			WriteTimeout: config.HTTP.WriteTimeout,
		},
		deliveryUseCase: deliveryUseCase,
		deliveryRepo:    deliveryRepo,
		config:          config,
		db:              db,
		router:          router,
		sagaConsumer:    sagaConsumer,
		rabbitMQ:        rabbitMQ,
	}, nil
}

// Run запускает приложение
func (a *App) Run() error {
	// Настраиваем и запускаем обработчик сообщений саги (резервирование/компенсация)
	if err := a.sagaConsumer.Setup(); err != nil {
		return fmt.Errorf("ошибка настройки основного consumer'а саги: %w", err)
	}

	// Настраиваем и запускаем обработчик сообщений саги (подтверждение)
	if err := a.sagaConsumer.SetupConfirmConsumer(); err != nil {
		return fmt.Errorf("ошибка настройки consumer'а подтверждения саги: %w", err)
	}

	// Запускаем HTTP сервер
	go func() {
		log.Printf("HTTP сервер запущен на порту %s", a.config.HTTP.Port)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	// Обрабатываем сигналы останова
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Завершение работы сервера...")

	// Даем 5 секунд на завершение всех запросов
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Закрываем HTTP сервер
	if err := a.httpServer.Shutdown(ctx); err != nil {
		log.Fatalf("Ошибка при завершении работы сервера: %v", err)
	}

	// Закрываем соединение с RabbitMQ
	if err := a.rabbitMQ.Close(); err != nil {
		log.Printf("Ошибка при закрытии соединения с RabbitMQ: %v", err)
	}

	log.Println("Сервер успешно остановлен")
	return nil
}

// initDB инициализирует подключение к базе данных
func initDB(config *config.Config) (*gorm.DB, error) {
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.Postgres.Host,
		config.Postgres.Port,
		config.Postgres.User,
		config.Postgres.Password,
		config.Postgres.DBName,
		config.Postgres.SSLMode,
	)

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	return db, nil
}

// initRabbitMQ инициализирует подключение к RabbitMQ
func initRabbitMQ(config *config.Config) (*pkgRabbitMQ.RabbitMQ, error) {
	rabbitConfig := pkgRabbitMQ.Config{
		User:     config.RabbitMQ.User,
		Password: config.RabbitMQ.Password,
		Host:     config.RabbitMQ.Host,
		Port:     config.RabbitMQ.Port,
		VHost:    config.RabbitMQ.VHost,
	}

	rabbitMQ, err := pkgRabbitMQ.NewRabbitMQ(rabbitConfig)
	if err != nil {
		return nil, err
	}

	return rabbitMQ, nil
}
