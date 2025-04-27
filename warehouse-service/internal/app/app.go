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

	"github.com/director74/dz8_shop/pkg/auth"
	"github.com/director74/dz8_shop/pkg/database"
	"github.com/director74/dz8_shop/pkg/errors"
	"github.com/director74/dz8_shop/pkg/messaging"
	"github.com/director74/dz8_shop/pkg/rabbitmq"
	"github.com/director74/dz8_shop/warehouse-service/config"
	httpController "github.com/director74/dz8_shop/warehouse-service/internal/controller/http"
	rmqController "github.com/director74/dz8_shop/warehouse-service/internal/controller/rabbitmq"
	"github.com/director74/dz8_shop/warehouse-service/internal/entity"
	"github.com/director74/dz8_shop/warehouse-service/internal/repo"
	"github.com/director74/dz8_shop/warehouse-service/internal/usecase"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// App представляет основное приложение сервиса склада
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
	if err := database.AutoMigrateWithCleanup(db, &entity.WarehouseItem{}, &entity.WarehouseReservation{}); err != nil {
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
		"warehouse_events": "topic",
		"order_events":     "topic",
	}
	queues := map[string]map[string]string{
		"order_warehouse_queue": {
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

	// Создание репозитория склада
	warehouseRepo := repo.NewWarehouseRepo(db)

	// Создание use case склада
	warehouseUseCase := usecase.NewWarehouseUseCase(warehouseRepo)

	// Создание обработчика HTTP запросов
	warehouseHandler := httpController.NewWarehouseHandler(warehouseUseCase, cfg)

	// Проверяем, что RabbitMQ имеет правильный тип
	rawRMQ, ok := rmq.(*rabbitmq.RabbitMQ)
	if !ok {
		database.CloseDB(db)
		rmq.Close()
		return nil, fmt.Errorf("неожиданный тип для RabbitMQ: %T", rmq)
	}

	// Создание обработчика сообщений RabbitMQ
	sagaConsumer := rmqController.NewSagaConsumer(warehouseUseCase, rawRMQ)

	// Регистрация маршрутов
	warehouseHandler.RegisterRoutes(router, authMiddleware.AuthRequired())

	// Настройка обработки сообщений RabbitMQ
	if err := sagaConsumer.Setup(); err != nil {
		database.CloseDB(db)
		rmq.Close()
		return nil, errors.AppendPrefix(err, "ошибка настройки обработчика сообщений")
	}

	// Настройка HTTP сервера
	server := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.HTTP.Port),
		Handler:      router,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
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
			log.Printf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	log.Printf("Сервис склада запущен на порту %s", a.config.HTTP.Port)

	// Ожидание сигнала для грациозного завершения
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Завершение работы сервиса склада...")

	// Завершение HTTP сервера
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := a.server.Shutdown(ctx); err != nil {
		log.Printf("Ошибка остановки HTTP сервера: %v", err)
	}

	// Закрытие соединения с RabbitMQ
	if err := a.rabbitMQ.Close(); err != nil {
		log.Printf("Ошибка закрытия соединения с RabbitMQ: %v", err)
	}

	log.Println("Сервис склада остановлен")
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
