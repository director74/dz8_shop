package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/pkg/sagahandler"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Мок для OrderRepository
type MockOrderRepository struct {
	mock.Mock
}

func (m *MockOrderRepository) Create(ctx context.Context, order *entity.Order) error {
	args := m.Called(ctx, order)
	// Имитируем установку ID для заказа, как это делает реальная БД
	if order.ID == 0 {
		order.ID = 10 // Тестовый ID
	}
	return args.Error(0)
}

func (m *MockOrderRepository) GetByID(ctx context.Context, id uint) (*entity.Order, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.Order), args.Error(1)
}

func (m *MockOrderRepository) Update(ctx context.Context, order *entity.Order) error {
	args := m.Called(ctx, order)
	return args.Error(0)
}

func (m *MockOrderRepository) UpdateOrderStatus(ctx context.Context, orderID uint, status entity.OrderStatus) error {
	args := m.Called(ctx, orderID, status)
	return args.Error(0)
}

// Мок для SagaStateRepository
type MockSagaStateRepository struct {
	mock.Mock
}

func (m *MockSagaStateRepository) Create(ctx context.Context, state *entity.SagaState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockSagaStateRepository) GetByID(ctx context.Context, sagaID string) (*entity.SagaState, error) {
	args := m.Called(ctx, sagaID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*entity.SagaState), args.Error(1)
}

func (m *MockSagaStateRepository) Update(ctx context.Context, state *entity.SagaState) error {
	args := m.Called(ctx, state)
	return args.Error(0)
}

func (m *MockSagaStateRepository) Delete(ctx context.Context, sagaID string) error {
	args := m.Called(ctx, sagaID)
	return args.Error(0)
}

// Мок для SagaRabbitMQClient
type MockRabbitMQ struct {
	mock.Mock
	PublishHistory []PublishData // История вызовов PublishMessage для проверки
}

type PublishData struct {
	Exchange   string
	RoutingKey string
	Message    interface{}
}

func (m *MockRabbitMQ) PublishMessage(exchange, routingKey string, message interface{}) error {
	log.Printf("[MOCK] PublishMessage: exchange=%s, routingKey=%s", exchange, routingKey)
	args := m.Called(exchange, routingKey, message)

	// Сохраняем данные для проверки
	m.PublishHistory = append(m.PublishHistory, PublishData{
		Exchange:   exchange,
		RoutingKey: routingKey,
		Message:    message,
	})

	return args.Error(0)
}

// Расширенный мок для RabbitMQ, который реализует методы для SetupOrderSagaConsumer
func (m *MockRabbitMQ) DeclareExchange(name string, kind string) error {
	args := m.Called(name, kind)
	return args.Error(0)
}

func (m *MockRabbitMQ) DeclareQueue(name string) error {
	args := m.Called(name)
	return args.Error(0)
}

func (m *MockRabbitMQ) BindQueue(queueName, exchangeName, routingKey string) error {
	args := m.Called(queueName, exchangeName, routingKey)
	return args.Error(0)
}

func (m *MockRabbitMQ) ConsumeMessages(queueName, consumerName string, handler func([]byte) error) error {
	args := m.Called(queueName, consumerName, handler)
	return args.Error(0)
}

// Вспомогательная функция для создания тестового сообщения саги
func createSagaMessage(sagaID, stepName string, operation sagahandler.SagaOperation, status sagahandler.SagaStatus, sagaData interface{}) ([]byte, error) {
	dataBytes, err := json.Marshal(sagaData)
	if err != nil {
		return nil, err
	}

	message := sagahandler.SagaMessage{
		SagaID:    sagaID,
		StepName:  stepName,
		Operation: operation,
		Status:    status,
		Data:      dataBytes,
		Timestamp: time.Now().Unix(),
	}

	return json.Marshal(message)
}

// Тестовые данные
func createTestSagaData() *sagahandler.SagaData {
	return &sagahandler.SagaData{
		OrderID: 10,
		UserID:  5,
		Items: []sagahandler.OrderItem{
			{
				ProductID: 1,
				Quantity:  2,
				Price:     100,
			},
		},
		Amount: 200,
		Status: "pending",
	}
}

// Вспомогательная функция для создания тестового заказа
func createTestOrder() *entity.Order {
	return &entity.Order{
		ID:     10,
		UserID: 5,
		Items: []entity.OrderItem{
			{
				ProductID: 1,
				Quantity:  2,
				Price:     100,
			},
		},
		Amount:    200,
		Status:    entity.OrderStatusPending,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// Основные тесты для оркестратора

// TestStartOrderSaga тестирует запуск саги для обработки заказа
func TestStartOrderSaga(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовые данные
	orderData := createTestSagaData()

	// Настраиваем ожидаемое поведение репозитория
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Order")).Return(nil)
	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.SagaState")).Return(nil)
	mockStateRepo.On("Update", mock.Anything, mock.AnythingOfType("*entity.SagaState")).Return(nil)

	// Настраиваем ожидаемое поведение RabbitMQ
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_billing.execute", mock.Anything).Return(nil)

	// Вызываем тестируемый метод
	err := orchestrator.StartOrderSaga(context.Background(), orderData)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)

	// Проверяем, что сообщение было отправлено в правильную очередь
	assert.Equal(t, 1, len(mockRabbitMQ.PublishHistory))
	assert.Equal(t, "saga_exchange", mockRabbitMQ.PublishHistory[0].Exchange)
	assert.Equal(t, "saga.process_billing.execute", mockRabbitMQ.PublishHistory[0].RoutingKey)
}

// TestHandleSagaResult_SuccessExecution тестирует успешное выполнение шага саги
func TestHandleSagaResult_SuccessExecution(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовый заказ и данные саги
	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusRunning}

	// Создаем тестовое сообщение (успешное выполнение шага billing)
	testMessage, err := createSagaMessage(sagaID, "process_billing", sagahandler.OperationExecute, sagahandler.StatusCompleted, sagaData)
	assert.NoError(t, err)

	// Настраиваем ожидаемое поведение репозитория
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	// Добавляем ожидание вызова Update с правильным статусом Pending
	mockRepo.On("Update", mock.Anything, mock.MatchedBy(func(order *entity.Order) bool {
		return order.ID == 10 && order.Status == entity.OrderStatusPending
	})).Return(nil)
	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil)
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		return state.SagaID == sagaID && state.LastStep == "process_billing" && state.Status == entity.SagaStatusRunning
	})).Return(nil)

	// Настраиваем ожидаемое поведение RabbitMQ для следующего шага
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_payment.execute", mock.Anything).Return(nil)

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessage)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)

	// Проверяем, что сообщение для следующего шага было отправлено
	assert.Equal(t, 1, len(mockRabbitMQ.PublishHistory))
	assert.Equal(t, "saga.process_payment.execute", mockRabbitMQ.PublishHistory[0].RoutingKey)
}

// TestHandleSagaResult_FailedExecution тестирует обработку неудачного выполнения шага саги
func TestHandleSagaResult_FailedExecution(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовый заказ и данные саги
	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusRunning, CompensatedSteps: make(map[string]interface{})}

	// Создаем тестовое сообщение (неудачное выполнение шага payment)
	testMessage, err := createSagaMessage(sagaID, "process_payment", sagahandler.OperationExecute, sagahandler.StatusFailed, sagaData)
	assert.NoError(t, err)

	// Настраиваем ожидаемое поведение репозитория
	// Добавляем ожидание GetByID, которое происходит перед UpdateOrderStatus
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusFailed).Return(nil)
	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil).Once()
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		return state.SagaID == sagaID && state.Status == entity.SagaStatusCompensating && state.LastStep == "process_payment"
	})).Return(nil).Once()
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil).Once()
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		return state.SagaID == sagaID && state.Status == entity.SagaStatusCompensating && state.TotalToCompensate == 1
	})).Return(nil).Once()

	// Настраиваем ожидаемое поведение RabbitMQ для компенсации предыдущего шага
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_billing.compensate", mock.Anything).Return(nil)

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessage)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)

	// Проверяем, что сообщение для компенсации было отправлено
	mockRabbitMQ.AssertCalled(t, "PublishMessage", "saga_exchange", "saga.process_billing.compensate", mock.Anything)
}

// TestHandleSagaResult_CompensationResult тестирует корректную обработку компенсации
func TestHandleSagaResult_CompensationResult(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовый заказ и данные саги
	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	// Устанавливаем правильное TotalToCompensate = 4
	testSagaState := &entity.SagaState{
		SagaID:            sagaID,
		OrderID:           10,
		Status:            entity.SagaStatusCompensating,
		CompensatedSteps:  make(map[string]interface{}),
		TotalToCompensate: 4, // Ожидаем компенсацию 4 шагов
	}

	// Настраиваем ожидаемое поведение репозитория
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil).Maybe()                     // Может вызываться или нет в compensate
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCancelled).Return(nil).Maybe() // Может вызываться или нет
	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil)

	// Ожидаем Update после каждого шага компенсации
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// После reserve_delivery
		_, compensated := state.CompensatedSteps["reserve_delivery"].(bool)
		return state.SagaID == sagaID && state.LastStep == "reserve_delivery" && state.Status == entity.SagaStatusCompensating && compensated && len(state.CompensatedSteps) == 1
	})).Return(nil).Once() // .Once() для первого вызова

	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// После reserve_warehouse
		_, c1 := state.CompensatedSteps["reserve_delivery"].(bool)
		_, c2 := state.CompensatedSteps["reserve_warehouse"].(bool)
		return state.SagaID == sagaID && state.LastStep == "reserve_warehouse" && state.Status == entity.SagaStatusCompensating && c1 && c2 && len(state.CompensatedSteps) == 2
	})).Return(nil).Once() // .Once() для второго вызова

	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// После process_payment
		_, c1 := state.CompensatedSteps["reserve_delivery"].(bool)
		_, c2 := state.CompensatedSteps["reserve_warehouse"].(bool)
		_, c3 := state.CompensatedSteps["process_payment"].(bool)
		return state.SagaID == sagaID && state.LastStep == "process_payment" && state.Status == entity.SagaStatusCompensating && c1 && c2 && c3 && len(state.CompensatedSteps) == 3
	})).Return(nil).Once() // .Once() для третьего вызова

	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// После process_billing (ФИНАЛЬНЫЙ)
		_, c1 := state.CompensatedSteps["reserve_delivery"].(bool)
		_, c2 := state.CompensatedSteps["reserve_warehouse"].(bool)
		_, c3 := state.CompensatedSteps["process_payment"].(bool)
		_, c4 := state.CompensatedSteps["process_billing"].(bool)
		return state.SagaID == sagaID && state.LastStep == "process_billing" && state.Status == entity.SagaStatusCompensated && c1 && c2 && c3 && c4 && len(state.CompensatedSteps) == 4
	})).Return(nil).Once() // .Once() для четвертого вызова

	mockStateRepo.On("Delete", mock.Anything, sagaID).Return(nil).Once() // Ожидаем Delete после последнего Update

	// 1. Компенсация reserve_delivery
	testMessage, err := createSagaMessage(sagaID, "reserve_delivery", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)
	err = orchestrator.HandleSagaResult(testMessage)
	assert.NoError(t, err)

	// 2. Компенсация reserve_warehouse
	testMessage2, err := createSagaMessage(sagaID, "reserve_warehouse", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)
	err = orchestrator.HandleSagaResult(testMessage2)
	assert.NoError(t, err)

	// 3. Компенсация process_payment
	testMessage3, err := createSagaMessage(sagaID, "process_payment", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)
	err = orchestrator.HandleSagaResult(testMessage3)
	assert.NoError(t, err)

	// 4. Компенсация process_billing
	testMessage4, err := createSagaMessage(sagaID, "process_billing", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)
	err = orchestrator.HandleSagaResult(testMessage4)
	assert.NoError(t, err)

	// Проверяем ожидания моков
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	// Проверяем, что PublishMessage не вызывался в этом тесте
	mockRabbitMQ.AssertExpectations(t)
}

// TestHandleSagaResult_CompleteOrder тестирует завершение обработки заказа
func TestHandleSagaResult_CompleteOrder(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовый заказ и данные саги
	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusCompleted}

	// Создаем тестовое сообщение (успешное завершение последнего шага)
	testMessage, err := createSagaMessage(sagaID, "complete_order", sagahandler.OperationExecute, sagahandler.StatusCompleted, sagaData)
	assert.NoError(t, err)

	// Настраиваем ожидаемое поведение репозитория
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCompleted).Return(nil)
	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil)
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		return state.SagaID == sagaID && state.Status == entity.SagaStatusCompleted
	})).Return(nil)
	mockStateRepo.On("Delete", mock.Anything, sagaID).Return(nil)

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessage)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)

	// Проверяем, что новых сообщений не было отправлено
	assert.Equal(t, 0, len(mockRabbitMQ.PublishHistory))
}

// TestHandleSagaResult_ExecuteCompensated тестирует обработку сообщения с операцией execute и статусом compensated
func TestHandleSagaResult_ExecuteCompensated(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовый заказ и данные саги
	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusRunning, CompensatedSteps: make(map[string]interface{})}

	// Создаем тестовое сообщение (execute со статусом compensated от delivery)
	testMessage, err := createSagaMessage(sagaID, "reserve_delivery", sagahandler.OperationExecute, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)

	// Настраиваем ожидаемое поведение репозитория
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusFailed).Return(nil)
	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil).Once()
	// Первый Update: сохраняем статус Compensating и ошибку
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		return state.SagaID == sagaID && state.Status == entity.SagaStatusCompensating
	})).Return(nil).Once()
	// Второй GetByID: вызывается внутри startCompensationProcess
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil).Once()
	// Второй Update: устанавливаем TotalToCompensate
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// Рассчитываем ожидаемое количество шагов для компенсации перед reserve_delivery
		// process_billing, process_payment, reserve_warehouse - всего 3
		return state.SagaID == sagaID && state.Status == entity.SagaStatusCompensating && state.TotalToCompensate == 3
	})).Return(nil).Once()

	// Настраиваем ожидаемое поведение RabbitMQ для компенсации предыдущих шагов
	// Ожидаем компенсацию 3 шагов: process_billing, process_payment, reserve_warehouse
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.reserve_warehouse.compensate", mock.Anything).Return(nil)
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_payment.compensate", mock.Anything).Return(nil)
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_billing.compensate", mock.Anything).Return(nil)

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessage)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)

	// Проверяем, что сообщения для компенсации предыдущих шагов были отправлены
	assert.Equal(t, 3, len(mockRabbitMQ.PublishHistory)) // Ожидаем 3 сообщения
	actualKeys := map[string]bool{}
	for _, pub := range mockRabbitMQ.PublishHistory {
		actualKeys[pub.RoutingKey] = true
	}
	expectedKeys := map[string]bool{
		"saga.reserve_warehouse.compensate": true,
		"saga.process_payment.compensate":   true,
		"saga.process_billing.compensate":   true,
	}
	for key := range expectedKeys {
		assert.True(t, actualKeys[key], "Ожидалась публикация компенсации для %s", key)
	}
}

// TestSetupOrderSagaConsumer тестирует настройку обработчика сообщений саги
func TestSetupOrderSagaConsumer(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := new(MockRabbitMQ)
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Настраиваем ожидаемое поведение RabbitMQ
	mockRabbitMQ.On("DeclareExchange", "saga_exchange", "topic").Return(nil)
	mockRabbitMQ.On("DeclareQueue", "order_service.saga_results").Return(nil)
	mockRabbitMQ.On("BindQueue", "order_service.saga_results", "saga_exchange", "saga.*.result").Return(nil)
	mockRabbitMQ.On("ConsumeMessages", "order_service.saga_results", "order_saga_result_consumer", mock.AnythingOfType("func([]uint8) error")).Return(nil)

	// Вызываем тестируемый метод
	err := orchestrator.SetupOrderSagaConsumer()

	// Проверяем результаты
	assert.NoError(t, err)
	mockRabbitMQ.AssertExpectations(t)
}

// Добавляем тесты для обработки ошибок

// TestStartOrderSaga_CreateError тестирует ошибку при создании заказа
func TestStartOrderSaga_CreateError(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовые данные
	orderData := createTestSagaData()

	// Настраиваем ожидаемое поведение репозитория с ошибкой
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*entity.Order")).Return(fmt.Errorf("database error"))

	// Вызываем тестируемый метод
	err := orchestrator.StartOrderSaga(context.Background(), orderData)

	// Проверяем результаты
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "ошибка при создании заказа")
	mockRepo.AssertExpectations(t)

	// Проверяем, что сообщения не отправлялись
	assert.Equal(t, 0, len(mockRabbitMQ.PublishHistory))
}

// TestHandleSagaResult_GetOrderError тестирует ошибку при получении заказа
func TestHandleSagaResult_GetOrderError(t *testing.T) {
	// Создаем моки
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	// Создаем оркестратор с моками
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// Тестовые данные саги
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"

	// Создаем тестовое сообщение
	testMessage, err := createSagaMessage(sagaID, "process_billing", sagahandler.OperationExecute, sagahandler.StatusCompleted, sagaData)
	assert.NoError(t, err)

	// Настраиваем ожидаемое поведение репозитория состояний с ошибкой
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(nil, fmt.Errorf("Ошибка получения состояния саги из БД: saga state not found"))

	// Настраиваем ожидаемое поведение репозитория с ошибкой
	// mockRepo.On("GetByID", mock.Anything, uint(10)).Return(nil, fmt.Errorf("order not found")) // Этот мок не нужен здесь, так как GetByID для Order не вызывается, если GetByID для SagaState вернул ошибку

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessage)

	// Проверяем результаты
	assert.Error(t, err)
	// Ожидаем ошибку получения состояния саги (как она форматируется в HandleSagaResult)
	assert.Contains(t, err.Error(), "Ошибка получения состояния саги из БД: saga state not found")
	mockStateRepo.AssertExpectations(t)
	mockRepo.AssertExpectations(t) // Проверяем, что GetByID для Order не вызывался

	// Проверяем, что сообщения не отправлялись
	assert.Equal(t, 0, len(mockRabbitMQ.PublishHistory))
}

// TestHandleSagaResult_UpdateOrderError тестирует ошибку при обновлении заказа
func TestHandleSagaResult_UpdateOrderError(t *testing.T) {
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusRunning, CompensatedSteps: make(map[string]interface{})}

	// Гарантируем, что предыдущий шаг не компенсирован
	sagaData.CompensatedSteps = map[string]bool{}

	testMessage, err := createSagaMessage(sagaID, "process_payment", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)

	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	// Настраиваем мок UpdateOrderStatus на возврат ошибки
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCancelled).Return(fmt.Errorf("database update error")) // Ожидаем Canceled, т.к. это compensate/compensated
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil)
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// Этот Update должен быть вызван до UpdateOrderStatus, чтобы пометить шаг компенсированным
		_, compensated := state.CompensatedSteps["process_payment"].(bool)
		return state.SagaID == sagaID && compensated && state.LastStep == "process_payment"
	})).Return(nil)
	// PublishMessage не должен вызываться при ошибке UpdateOrderStatus
	// mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_billing.compensate", mock.Anything).Return(nil)
	// Delete не должен вызываться при ошибке
	// mockStateRepo.On("Delete", mock.Anything, sagaID).Return(nil)

	err = orchestrator.HandleSagaResult(testMessage)
	assert.Error(t, err) // Ожидаем ошибку
	// Проверяем исходную ошибку, возвращенную моком
	assert.EqualError(t, err, "database update error")
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)
}

// TestHandleSagaResult_IgnoreDuplicateCompensated тестирует игнорирование повторных сообщений compensate/compensated для уже компенсированного шага
func TestHandleSagaResult_IgnoreDuplicateCompensated(t *testing.T) {
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"

	// Помечаем шаг как уже компенсированный только в sagaData
	sagaData.CompensatedSteps = map[string]bool{"process_payment": true}

	// Создаем тестовое сообщение (повторное compensate/compensated для process_payment)
	testMessage, err := createSagaMessage(sagaID, "process_payment", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)

	// Не требуется мокать Update, только GetByID (для совместимости с оркестратором)
	// Мокаем GetByID, так как он вызывается в начале HandleSagaResult
	initialState := &entity.SagaState{
		SagaID:            sagaID,
		OrderID:           10,
		Status:            entity.SagaStatusCompensating,
		CompensatedSteps:  map[string]interface{}{"process_payment": true}, // Шаг уже помечен как компенсированный
		TotalToCompensate: 1,                                               // Не важно для этого теста, но должно быть > 0
	}
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(initialState, nil)
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)

	err = orchestrator.HandleSagaResult(testMessage)
	assert.NoError(t, err)
	// Проверяем, что не было публикаций новых сообщений компенсации
	assert.Equal(t, 0, len(mockRabbitMQ.PublishHistory))
}

// Тест: повторное сообщение compensate/compensated для шага, который еще не был компенсирован (должна быть инициирована компенсация)
func TestHandleSagaResult_CompensateNotYetCompensated(t *testing.T) {
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	sagaID := "saga-order-10-123456789"
	// Тестовое состояние, где шаг process_billing еще не компенсирован
	testSagaState := &entity.SagaState{
		SagaID:            sagaID,
		OrderID:           10,
		Status:            entity.SagaStatusCompensating,
		CompensatedSteps:  map[string]interface{}{"process_payment": true}, // payment уже компенсирован
		TotalToCompensate: 2,                                               // Ожидаем billing и payment
	}
	sagaData := createTestSagaData()

	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCancelled).Return(nil)
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil)
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		_, billingComp := state.CompensatedSteps["process_billing"].(bool)
		return state.SagaID == sagaID &&
			state.Status == entity.SagaStatusCompensated && // Статус должен стать Compensated
			billingComp &&
			len(state.CompensatedSteps) == 2 // Оба шага компенсированы
	})).Return(nil)
	mockStateRepo.On("Delete", mock.Anything, sagaID).Return(nil)

	// Моделируем приход сообщения compensate/compensated для process_billing
	testMessage, err := createSagaMessage(sagaID, "process_billing", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)
	err = orchestrator.HandleSagaResult(testMessage)
	assert.NoError(t, err)

	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)
}

// Тест: сообщение compensate/compensated для несуществующего шага (должно быть корректно обработано, без паники и публикаций)
func TestHandleSagaResult_CompensateUnknownStep(t *testing.T) {
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	// testOrder := createTestOrder() // Не используется
	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusCompensating}

	// Создаем тестовое сообщение (compensate/compensated для несуществующего шага)
	testMessage, err := createSagaMessage(sagaID, "unknown_step", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)

	// Настраиваем ожидаемое поведение репозитория состояний
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil)
	// Update ДОЛЖЕН вызываться, так как HandleSagaResult помечает шаг компенсированным
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		_, compensated := state.CompensatedSteps["unknown_step"].(bool)
		return state.SagaID == sagaID && compensated && state.LastStep == "unknown_step"
	})).Return(nil)
	// GetByID для Order ДОЛЖЕН вызываться в блоке compensate/compensated
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	// UpdateOrderStatus ДОЛЖЕН вызываться, так как статус заказа != Canceled
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCancelled).Return(nil)

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessage)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)
}

// Тест: конкурентная обработка сообщений compensate/compensated для одного шага
func TestHandleSagaResult_ConcurrentCompensate(t *testing.T) {
	// Этот тест в текущем виде проверяет скорее идемпотентность обработки
	// сообщения compensate/compensated, а не гонку за внутренним состоянием.
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)

	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	sagaID := "saga-order-10-123456789"
	sagaData := createTestSagaData()
	// Начальное состояние: компенсируется, 1 шаг ожидается, 0 компенсировано
	initialState := &entity.SagaState{
		SagaID:            sagaID,
		OrderID:           10,
		Status:            entity.SagaStatusCompensating,
		CompensatedSteps:  make(map[string]interface{}),
		TotalToCompensate: 1,
	}
	// Состояние после первой обработки: компенсируется, 1 шаг ожидается, 1 компенсирован
	firstUpdateState := &entity.SagaState{
		SagaID:            sagaID,
		OrderID:           10,
		Status:            entity.SagaStatusCompensated, // Сразу станет compensated
		CompensatedSteps:  map[string]interface{}{"process_billing": true},
		TotalToCompensate: 1,
	}

	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCancelled).Return(nil)
	// Первый вызов GetByID вернет initial state
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(initialState, nil).Once()
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		_, compensated := state.CompensatedSteps["process_billing"].(bool)
		return state.SagaID == sagaID && state.Status == entity.SagaStatusCompensated && compensated
	})).Return(nil).Once() // Первый Update
	mockStateRepo.On("Delete", mock.Anything, sagaID).Return(nil).Once() // Ожидаем удаление после первого успешного Update
	// Второй вызов GetByID вернет уже обновленное состояние (или то же самое, если гонка)
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(firstUpdateState, nil).Maybe() // Может быть вызван или нет, если гонка

	// Создаем сообщение
	testMessage, err := createSagaMessage(sagaID, "process_billing", sagahandler.OperationCompensate, sagahandler.StatusCompensated, sagaData)
	assert.NoError(t, err)

	// Запускаем обработку параллельно
	var wg sync.WaitGroup
	numGoroutines := 5
	wg.Add(numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func() {
			defer wg.Done()
			// Копируем слайс байт напрямую
			msgCopy := make([]byte, len(testMessage))
			copy(msgCopy, testMessage)
			_ = orchestrator.HandleSagaResult(msgCopy) // Игнорируем ошибки для простоты теста на конкурентность
		}()
	}
	wg.Wait()

	// Проверяем, что Delete был вызван хотя бы один раз (но не более одного)
	mockStateRepo.AssertCalled(t, "Delete", mock.Anything, sagaID)
	mockStateRepo.AssertNumberOfCalls(t, "Delete", 1)
	mockRepo.AssertCalled(t, "UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusCancelled)
	mockStateRepo.AssertCalled(t, "Update", mock.Anything, mock.AnythingOfType("*entity.SagaState"))
}

// Тест на обработку сообщения compensate/failed от сервиса
func TestHandleSagaResult_CompensateFailedFromService(t *testing.T) {
	mockRepo := new(MockOrderRepository)
	mockStateRepo := new(MockSagaStateRepository)
	mockRabbitMQ := &MockRabbitMQ{PublishHistory: []PublishData{}}
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	orchestrator := NewSagaOrchestrator(mockRepo, mockStateRepo, mockRabbitMQ, "saga_exchange", logger)

	sagaData := createTestSagaData()
	sagaID := "saga-order-10-123456789"
	failedStep := "process_payment"
	errorMsg := "Ошибка при компенсации платежа"
	testSagaState := &entity.SagaState{SagaID: sagaID, OrderID: 10, Status: entity.SagaStatusCompensating, CompensatedSteps: make(map[string]interface{})}

	// Создаем тестовое сообщение (compensate/failed)
	testMessageBytes, err := createSagaMessage(sagaID, failedStep, sagahandler.OperationCompensate, sagahandler.StatusFailed, sagaData)
	assert.NoError(t, err)
	// Добавляем ошибку в JSON вручную, так как createSagaMessage её не добавляет для compensate/failed
	var tempMsg sagahandler.SagaMessage
	_ = json.Unmarshal(testMessageBytes, &tempMsg)
	tempMsg.Error = errorMsg
	testMessageBytes, _ = json.Marshal(tempMsg)

	// Настраиваем ожидания
	// GetByID вызывается в блоке compensate/failed перед UpdateOrderStatus
	mockRepo.On("GetByID", mock.Anything, uint(10)).Return(createTestOrder(), nil)
	mockRepo.On("UpdateOrderStatus", mock.Anything, uint(10), entity.OrderStatusFailed).Return(nil)
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil).Once() // Первый GetByID
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// Ожидаем обновление статуса на Compensating и запись ошибки
		return state.SagaID == sagaID &&
			state.Status == entity.SagaStatusCompensating &&
			state.ErrorMessage == errorMsg &&
			state.LastStep == failedStep
	})).Return(nil).Once()
	mockStateRepo.On("GetByID", mock.Anything, sagaID).Return(testSagaState, nil).Once() // GetByID в startCompensationProcess
	mockStateRepo.On("Update", mock.Anything, mock.MatchedBy(func(state *entity.SagaState) bool {
		// Ожидаем обновление TotalToCompensate
		return state.SagaID == sagaID && state.TotalToCompensate == 1
	})).Return(nil).Once()
	mockRabbitMQ.On("PublishMessage", "saga_exchange", "saga.process_billing.compensate", mock.Anything).Return(nil)

	// Вызываем тестируемый метод
	err = orchestrator.HandleSagaResult(testMessageBytes)

	// Проверяем результаты
	assert.NoError(t, err)
	mockRepo.AssertExpectations(t)
	mockStateRepo.AssertExpectations(t)
	mockRabbitMQ.AssertExpectations(t)
}
