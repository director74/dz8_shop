# Обработчик саги для сервиса склада (warehouse-service)

Данный документ описывает, как интегрировать механизм саг в сервис склада для обеспечения надежных распределенных транзакций.

## Обзор

Обработчик саги для сервиса склада отвечает за резервирование товаров на складе и, при необходимости, отмену резервирования в рамках распределенной транзакции.

## Требуемые структуры и интерфейсы

Для корректной работы обработчика необходимо реализовать следующие структуры и методы в вашем сервисе склада:

### Структуры

```go
// В entity/warehouse.go

// WarehouseReservationItem представляет товар для резервирования
type WarehouseReservationItem struct {
    ProductID uint   // Идентификатор товара
    Quantity  int    // Количество товара для резервирования
}

// WarehouseReservation представляет запрос на резервирование товаров на складе
type WarehouseReservation struct {
    ID           uint                      // Уникальный идентификатор резервирования
    OrderID      uint                      // Идентификатор заказа
    UserID       uint                      // Идентификатор пользователя
    Items        []WarehouseReservationItem // Товары для резервирования
    Status       ReservationStatus         // Статус резервирования
    ExpiresAt    time.Time                 // Время истечения резервирования
    CreatedAt    time.Time                 // Время создания
    UpdatedAt    time.Time                 // Время последнего обновления
}

// ReservationStatus представляет статус резервирования
type ReservationStatus string

const (
    ReservationStatusPending   ReservationStatus = "pending"   // Ожидает обработки
    ReservationStatusConfirmed ReservationStatus = "confirmed" // Подтверждено
    ReservationStatusReleased  ReservationStatus = "released"  // Освобождено
    ReservationStatusExpired   ReservationStatus = "expired"   // Истекло
)
```

### Сервисный слой (usecase)

В сервисном слое необходимо реализовать следующие методы:

```go
// В usecase/warehouse_usecase.go

// Метод для резервирования товаров на складе
func (u *WarehouseUseCase) ReserveItems(reservation *entity.WarehouseReservation) (*entity.WarehouseReservation, error) {
    // Реализация резервирования товаров
    // ...
    return reservation, nil
}

// Метод для освобождения резервирования
func (u *WarehouseUseCase) ReleaseReservation(reservationID uint) error {
    // Реализация освобождения резервирования
    // ...
    return nil
}

// Метод для подтверждения резервирования
func (u *WarehouseUseCase) ConfirmReservation(reservationID uint) error {
    // Реализация подтверждения резервирования
    // ...
    return nil
}
```

## Интеграция с основным приложением

Для интеграции обработчика саги с основным приложением добавьте следующий код в функцию инициализации вашего приложения:

```go
// В cmd/app/main.go или другом месте, где инициализируются компоненты

func initRabbitMQ(cfg *config.Config, warehouseUseCase *usecase.WarehouseUseCase) (*rabbitmq.RabbitMQ, error) {
    // Инициализация RabbitMQ
    rabbitMQClient, err := rabbitmq.NewRabbitMQ(rabbitmq.Config{
        Host:     cfg.RabbitMQ.Host,
        Port:     cfg.RabbitMQ.Port,
        User:     cfg.RabbitMQ.User,
        Password: cfg.RabbitMQ.Password,
        VHost:    cfg.RabbitMQ.VHost,
    })
    if err != nil {
        return nil, fmt.Errorf("ошибка инициализации RabbitMQ: %w", err)
    }

    // Создание и настройка обработчика саги
    sagaConsumer := rabbitmq.NewSagaConsumer(warehouseUseCase, rabbitMQClient)
    if err := sagaConsumer.Setup(); err != nil {
        return nil, fmt.Errorf("ошибка настройки обработчика саги: %w", err)
    }

    return rabbitMQClient, nil
}
```

## Тестирование

При тестировании обработчика саги рекомендуется создать моки для usecase и RabbitMQ:

```go
// В тестах

func TestSagaConsumer_HandleReserveWarehouse(t *testing.T) {
    // Создаем моки
    mockUseCase := mocks.NewMockWarehouseUseCase(t)
    mockRabbitMQ := mocks.NewMockRabbitMQ(t)

    // Настраиваем поведение моков
    mockUseCase.On("ReserveItems", mock.Anything).Return(&entity.WarehouseReservation{
        ID:      123,
        OrderID: 456,
        Status:  entity.ReservationStatusConfirmed,
    }, nil)

    mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_payment.execute", mock.Anything).Return(nil)

    // Создаем обработчик и тестируем
    consumer := rabbitmq.NewSagaConsumer(mockUseCase, mockRabbitMQ)
    
    // Готовим тестовые данные
    orderData := OrderData{
        OrderID: "456",
        UserID:  "789",
        Items: []OrderItem{
            {ProductID: "1", Quantity: 2},
        },
    }
    
    // Тестируем обработку
    // ...
}
```

## Рекомендации

1. Все операции с базой данных должны выполняться в рамках транзакций для обеспечения атомарности.
2. Обработчики должны быть идемпотентными, чтобы повторное выполнение операции не приводило к ошибкам.
3. Используйте логирование для отладки и мониторинга процесса выполнения саги.
4. Реализуйте обработку таймаутов и повторных попыток для повышения надежности.
5. Убедитесь, что ваша система может корректно обрабатывать частичные сбои и компенсационные операции. 