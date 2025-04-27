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

	"github.com/director74/dz8_shop/order-service/config"
	httpController "github.com/director74/dz8_shop/order-service/internal/controller/http"
	rabbitmqController "github.com/director74/dz8_shop/order-service/internal/controller/rabbitmq"
	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/order-service/internal/repo"
	"github.com/director74/dz8_shop/order-service/internal/usecase"
	"github.com/director74/dz8_shop/order-service/internal/usecase/webapi"
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
	jwtManager *auth.JWTManager
	db         *gorm.DB
	rabbitMQ   *rabbitmq.RabbitMQ
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

	// Автомиграция моделей, включая SagaState
	if err := database.AutoMigrateWithCleanup(db, &entity.User{}, &entity.Order{}, &entity.OrderItem{}, &entity.SagaState{}); err != nil {
		return nil, errors.AppendPrefix(err, "не удалось выполнить миграцию")
	}

	// Инициализируем подключение к RabbitMQ
	rmq, err = messaging.InitRabbitMQ(config.RabbitMQ)
	if err != nil {
		database.CloseDB(db)
		return nil, errors.AppendPrefix(err, "не удалось подключиться к RabbitMQ")
	}

	// Настраиваем exchanges и очереди в RabbitMQ
	exchanges := map[string]string{
		"order_events":  "topic",
		"saga_exchange": "topic",
	}
	queues := map[string]map[string]string{} // Нет очередей для привязки в этом сервисе

	if err := messaging.SetupExchangesAndQueues(rmq, exchanges, queues); err != nil {
		database.CloseDB(db)
		rmq.Close()
		return nil, errors.AppendPrefix(err, "ошибка при настройке RabbitMQ")
	}

	// Инициализируем JWT менеджер
	jwtConfig := auth.NewConfig(
		config.JWT.SigningKey,
	)
	jwtConfig.TokenTTL = config.JWT.TokenTTL
	jwtConfig.TokenIssuer = config.JWT.TokenIssuer
	jwtConfig.TokenAudiences = config.JWT.TokenAudiences
	jwtManager := auth.NewJWTManager(jwtConfig)

	// Создаем репозитории
	userRepo := repo.NewUserGormRepository(db)
	orderRepo := repo.NewOrderRepository(db)
	sagaStateRepo := repo.NewSagaStateRepository(db) // Создаем репозиторий состояний саг

	// Создаем клиент для биллинга
	billingClient := webapi.NewBillingClient(config.Services.BillingURL)

	// Создаем middleware для аутентификации
	authMiddleware := auth.NewAuthMiddleware(jwtManager)

	// Создаем use cases, передавая sagaStateRepo в OrderUseCase
	authUseCase := usecase.NewAuthUseCase(userRepo, jwtManager, billingClient)
	orderUseCase := usecase.NewOrderUseCase(orderRepo, userRepo, sagaStateRepo, billingClient, rmq, "order_events", "saga_exchange")

	// Создаем и настраиваем DeliveryConsumer
	deliveryConsumer := rabbitmqController.NewDeliveryConsumer(orderUseCase, orderRepo, rmq, nil)
	if err := deliveryConsumer.Setup(); err != nil {
		// Логгируем ошибку, но не останавливаем приложение, т.к. основной функционал может работать
		log.Printf("ВНИМАНИЕ: Ошибка при настройке DeliveryConsumer: %v", err)
	}

	// Создаем HTTP контроллеры
	authHandler := httpController.NewAuthHandler(authUseCase)
	orderHandler := httpController.NewOrderHandler(orderUseCase, authMiddleware)

	// Инициализируем Gin роутер
	router := gin.Default()

	// Добавляем middleware для обработки ошибок и восстановления после паники
	router.Use(errors.RecoveryMiddleware())
	router.Use(errors.ErrorMiddleware())

	// Настраиваем обработчики для 404 и 405 ошибок
	router.NoRoute(errors.NotFoundHandler())
	router.NoMethod(errors.MethodNotAllowedHandler())

	// Регистрируем эндпоинты
	authHandler.RegisterRoutes(router)
	orderHandler.RegisterRoutes(router)

	// Настраиваем HTTP сервер
	httpServer := &http.Server{
		Addr:         ":" + config.HTTP.Port,
		Handler:      router,
		ReadTimeout:  config.HTTP.ReadTimeout,
		WriteTimeout: config.HTTP.WriteTimeout,
	}

	return &App{
		config:     config,
		httpServer: httpServer,
		jwtManager: jwtManager,
		db:         db,
		rabbitMQ:   rmq,
	}, nil
}

// Run запускает приложение
func (a *App) Run() error {
	// Настраиваем обработку сигналов завершения
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Запускаем HTTP сервер в горутине
	go func() {
		log.Printf("HTTP сервер запущен на порту %s", a.config.HTTP.Port)
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
