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

	"github.com/director74/dz8_shop/billing-service/config"
	httpController "github.com/director74/dz8_shop/billing-service/internal/controller/http"
	rmqController "github.com/director74/dz8_shop/billing-service/internal/controller/rabbitmq"
	"github.com/director74/dz8_shop/billing-service/internal/repo"
	"github.com/director74/dz8_shop/billing-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/auth"
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
	rabbitMQ   *rabbitmq.RabbitMQ
	jwtManager *auth.JWTManager
}

func NewApp(config *config.Config) (*App, error) {
	var db *gorm.DB
	var rmq *rabbitmq.RabbitMQ
	var err error

	// Инициализируем подключение к PostgreSQL
	db, err = database.NewPostgresDB(config.Postgres)
	if err != nil {
		return nil, errors.AppendPrefix(err, "не удалось подключиться к базе данных")
	}

	// Автомиграция моделей
	/*if err := database.AutoMigrateWithCleanup(db, &entity.Account{}); err != nil {
		return nil, errors.AppendPrefix(err, "не удалось выполнить миграцию")
	}*/

	// Инициализируем подключение к RabbitMQ
	rmq, err = messaging.InitRabbitMQ(config.RabbitMQ)
	if err != nil {
		database.CloseDB(db)
		return nil, errors.AppendPrefix(err, "не удалось подключиться к RabbitMQ")
	}

	// Настраиваем exchanges и очереди в RabbitMQ
	exchanges := map[string]string{
		"billing_events": "topic",
		"order_events":   "topic",
		"saga_exchange":  "topic",
	}
	queues := map[string]map[string]string{
		"order_billing_queue": {
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
		SigningKey:     config.JWT.SigningKey,
		TokenTTL:       config.JWT.TokenTTL,
		TokenIssuer:    config.JWT.TokenIssuer,
		TokenAudiences: config.JWT.TokenAudiences,
	}
	jwtManager := auth.NewJWTManager(jwtConfig)

	// Создаем middleware для авторизации
	authMiddleware := auth.NewAuthMiddleware(jwtManager)

	// Создаем репозитории
	billingRepo := repo.NewBillingRepository(db)
	billingUseCase := usecase.NewBillingUseCase(billingRepo, rmq, "billing_events")

	// Настраиваем обработчик сообщений из очереди заказов
	err = rmq.ConsumeMessages("order_billing_queue", "billing-service", func(data []byte) error {
		return billingUseCase.HandleOrderCreatedEvent(data)
	})
	if err != nil {
		database.CloseDB(db)
		rmq.Close()
		return nil, errors.AppendPrefix(err, "ошибка при настройке обработчика сообщений")
	}

	// Создаем и настраиваем SagaConsumer
	sagaConsumer := rmqController.NewSagaConsumer(billingUseCase, rmq)
	go func() {
		if err := sagaConsumer.Setup(); err != nil {
			log.Printf("Ошибка при настройке обработчика саги для биллинга: %v", err)
		} else {
			log.Println("Обработчик саги для биллинга успешно настроен")
		}
	}()

	billingHandler := httpController.NewBillingHandler(billingUseCase, authMiddleware)

	// Инициализируем Gin роутер
	router := gin.Default()

	// Добавляем middleware для обработки ошибок и восстановления после паники
	router.Use(errors.RecoveryMiddleware())
	router.Use(errors.ErrorMiddleware())

	// Настраиваем обработчики для 404 и 405 ошибок
	router.NoRoute(errors.NotFoundHandler())
	router.NoMethod(errors.MethodNotAllowedHandler())

	// Регистрируем эндпоинты
	billingHandler.RegisterRoutes(router)

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
		rabbitMQ:   rmq,
		jwtManager: jwtManager,
	}, nil
}

// Run запускает приложение
func (a *App) Run() error {
	// Настраиваем обработку сигналов завершения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем HTTP сервер в горутине
	go func() {
		log.Printf("Сервис биллинга запущен на порту %s", a.config.HTTP.Port)
		if err := a.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Ошибка запуска HTTP сервера: %v", err)
		}
	}()

	// Ожидаем сигнал завершения
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

	// Закрываем соединение с базой данных
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
