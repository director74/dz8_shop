package http

import (
	"net/http"
	"strconv"

	"github.com/director74/dz8_shop/payment-service/config"
	"github.com/director74/dz8_shop/payment-service/internal/entity"
	"github.com/director74/dz8_shop/payment-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/auth"
	pkgMiddleware "github.com/director74/dz8_shop/pkg/middleware"
	"github.com/gin-gonic/gin"
)

// PaymentHandler обработчик HTTP запросов для платежей
type PaymentHandler struct {
	paymentUseCase *usecase.PaymentUseCase
	config         *config.Config
}

// NewPaymentHandler создает новый обработчик платежей
func NewPaymentHandler(paymentUseCase *usecase.PaymentUseCase, cfg *config.Config) *PaymentHandler {
	return &PaymentHandler{
		paymentUseCase: paymentUseCase,
		config:         cfg,
	}
}

// HealthCheck обрабатывает запрос на проверку работоспособности сервиса
func (h *PaymentHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// ProcessPayment обрабатывает платеж
func (h *PaymentHandler) ProcessPayment(c *gin.Context) {
	var req entity.PaymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Проверяем права доступа
	userID := auth.GetUserID(c)
	if userID == 0 || userID != req.UserID {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещен"})
		return
	}

	confirmation, err := h.paymentUseCase.ProcessPayment(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, confirmation)
}

// CancelPayment отменяет платеж
func (h *PaymentHandler) CancelPayment(c *gin.Context) {
	paymentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID платежа"})
		return
	}

	err = h.paymentUseCase.CancelPayment(uint(paymentID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "платеж успешно отменен"})
}

// GetPayment возвращает платеж по ID
func (h *PaymentHandler) GetPayment(c *gin.Context) {
	paymentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID платежа"})
		return
	}

	// Получаем данные о платеже из базы данных через use case
	payment, err := h.paymentUseCase.GetPaymentByID(uint(paymentID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "платеж не найден"})
		return
	}

	response := entity.GetPaymentResponse{
		ID:            payment.ID,
		OrderID:       payment.OrderID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		PaymentMethod: payment.PaymentMethod,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
		CreatedAt:     payment.CreatedAt,
		UpdatedAt:     payment.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// GetPaymentByOrderID возвращает платеж по ID заказа
func (h *PaymentHandler) GetPaymentByOrderID(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID заказа"})
		return
	}

	// Получаем данные о платеже из базы данных через use case
	payment, err := h.paymentUseCase.GetPaymentForOrder(uint(orderID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "платеж не найден"})
		return
	}

	response := entity.GetPaymentResponse{
		ID:            payment.ID,
		OrderID:       payment.OrderID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		PaymentMethod: payment.PaymentMethod,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
		CreatedAt:     payment.CreatedAt,
		UpdatedAt:     payment.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// GetUserPayments возвращает список платежей пользователя
func (h *PaymentHandler) GetUserPayments(c *gin.Context) {
	userID, err := strconv.ParseUint(c.Param("user_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID пользователя"})
		return
	}

	// Проверяем права доступа
	currentUserID := auth.GetUserID(c)
	if currentUserID == 0 || currentUserID != uint(userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещен"})
		return
	}

	// Получаем платежи пользователя
	payments, err := h.paymentUseCase.GetPaymentsByUserID(uint(userID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Формируем ответ
	var paymentResponses []entity.GetPaymentResponse
	for _, payment := range payments {
		paymentResponses = append(paymentResponses, entity.GetPaymentResponse{
			ID:            payment.ID,
			OrderID:       payment.OrderID,
			UserID:        payment.UserID,
			Amount:        payment.Amount,
			PaymentMethod: payment.PaymentMethod,
			Status:        payment.Status,
			TransactionID: payment.TransactionID,
			CreatedAt:     payment.CreatedAt,
			UpdatedAt:     payment.UpdatedAt,
		})
	}

	response := entity.ListPaymentsResponse{
		Payments: paymentResponses,
		Total:    int64(len(payments)),
	}

	c.JSON(http.StatusOK, response)
}

// InternalProcessPayment обрабатывает платеж, вызываемый внутренними сервисами
func (h *PaymentHandler) InternalProcessPayment(c *gin.Context) {
	var req entity.PaymentRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	confirmation, err := h.paymentUseCase.ProcessPayment(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, confirmation)
}

// InternalGetPaymentByOrderID возвращает платеж по ID заказа (для внутренних сервисов)
func (h *PaymentHandler) InternalGetPaymentByOrderID(c *gin.Context) {
	orderID, err := strconv.ParseUint(c.Param("order_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID заказа"})
		return
	}

	payment, err := h.paymentUseCase.GetPaymentForOrder(uint(orderID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if payment == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "платеж не найден"})
		return
	}

	response := entity.GetPaymentResponse{
		ID:            payment.ID,
		OrderID:       payment.OrderID,
		UserID:        payment.UserID,
		Amount:        payment.Amount,
		PaymentMethod: payment.PaymentMethod,
		Status:        payment.Status,
		TransactionID: payment.TransactionID,
		CreatedAt:     payment.CreatedAt,
		UpdatedAt:     payment.UpdatedAt,
	}

	c.JSON(http.StatusOK, response)
}

// InternalCancelPayment отменяет платеж (для внутренних сервисов)
func (h *PaymentHandler) InternalCancelPayment(c *gin.Context) {
	paymentID, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "неверный ID платежа"})
		return
	}

	err = h.paymentUseCase.CancelPayment(uint(paymentID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "платеж успешно отменен"})
}

// RegisterRoutes регистрирует маршруты для платежей
func (h *PaymentHandler) RegisterRoutes(router *gin.Engine, authMiddleware gin.HandlerFunc) {
	// Добавляем эндпоинт для проверки работоспособности сервиса
	router.GET("/health", h.HealthCheck)

	// Публичные API маршруты (с авторизацией)
	payments := router.Group("/api/v1/payments")
	{
		payments.GET("/:id", authMiddleware, h.GetPayment)
		payments.POST("/process", authMiddleware, h.ProcessPayment)
		payments.POST("/:id/cancel", authMiddleware, h.CancelPayment)
		payments.GET("/by-order/:order_id", authMiddleware, h.GetPaymentByOrderID)
		payments.GET("/by-customer/:user_id", authMiddleware, h.GetUserPayments)
	}

	// Внутренние API маршруты (с проверкой доступа для внутренних сервисов)
	// Эти маршруты не должны быть доступны извне через ingress
	internalAPIConfig := &pkgMiddleware.InternalAPIConfig{
		TrustedNetworks: h.config.Internal.TrustedNetworks,
		APIKeyEnvName:   h.config.Internal.APIKeyEnvName,
		DefaultAPIKey:   h.config.Internal.DefaultAPIKey,
		HeaderName:      h.config.Internal.HeaderName,
	}

	internalAuthMiddleware := pkgMiddleware.NewInternalAuthMiddleware(internalAPIConfig)
	internal := router.Group("/internal", internalAuthMiddleware.Required())
	{
		internalPayments := internal.Group("/payments")
		{
			internalPayments.POST("/process", h.InternalProcessPayment)
			internalPayments.POST("/:id/cancel", h.InternalCancelPayment)
			internalPayments.GET("/by-order/:order_id", h.InternalGetPaymentByOrderID)
		}
	}
}
