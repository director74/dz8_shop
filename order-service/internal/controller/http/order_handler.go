package http

import (
	"context"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/order-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/auth"
)

type OrderHandler struct {
	orderUseCase   *usecase.OrderUseCase
	authMiddleware *auth.AuthMiddleware
}

func NewOrderHandler(orderUseCase *usecase.OrderUseCase, authMiddleware *auth.AuthMiddleware) *OrderHandler {
	return &OrderHandler{
		orderUseCase:   orderUseCase,
		authMiddleware: authMiddleware,
	}
}

func (h *OrderHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.HealthCheck)

	api := router.Group("/api/v1")
	{
		// Публичные эндпоинты
		api.POST("/users", h.CreateUser)

		// Защищенные эндпоинты
		authorized := api.Group("")
		authorized.Use(h.authMiddleware.AuthRequired())
		{
			authorized.POST("/orders", h.CreateOrder)
			authorized.GET("/orders/:id", h.GetOrder)
			authorized.GET("/users/:id/orders", h.ListUserOrders)
		}
	}
}

func (h *OrderHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *OrderHandler) CreateUser(c *gin.Context) {
	var req entity.CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.orderUseCase.CreateUser(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *OrderHandler) CreateOrder(c *gin.Context) {
	var req entity.CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "пользователь не авторизован"})
		return
	}
	req.UserID = userID

	// Получаем JWT токен из контекста Gin
	jwtToken, exists := c.Get("jwt_token")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "отсутствует токен авторизации"})
		return
	}

	// Создаем новый контекст с JWT токеном
	ctx := context.WithValue(c.Request.Context(), "jwt_token", jwtToken)

	resp, err := h.orderUseCase.CreateOrder(ctx, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *OrderHandler) GetOrder(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID"})
		return
	}

	// Здесь стоит добавить проверку, что заказ принадлежит текущему пользователю
	// Но для простоты позволим любому авторизованному пользователю получить любой заказ

	resp, err := h.orderUseCase.GetOrder(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *OrderHandler) ListUserOrders(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID пользователя"})
		return
	}

	// Проверяем, что пользователь запрашивает свои заказы
	currentUserID := auth.GetUserID(c)
	if currentUserID != uint(userID) {
		c.JSON(http.StatusForbidden, gin.H{"error": "доступ запрещен"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.orderUseCase.ListUserOrders(c.Request.Context(), uint(userID), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
