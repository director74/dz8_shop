package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/director74/dz8_shop/billing-service/internal/entity"
	"github.com/director74/dz8_shop/billing-service/internal/usecase"
	"github.com/director74/dz8_shop/pkg/auth"
)

type BillingHandler struct {
	billingUseCase *usecase.BillingUseCase
	authMiddleware *auth.AuthMiddleware
}

func NewBillingHandler(billingUseCase *usecase.BillingUseCase, authMiddleware *auth.AuthMiddleware) *BillingHandler {
	return &BillingHandler{
		billingUseCase: billingUseCase,
		authMiddleware: authMiddleware,
	}
}

func (h *BillingHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.HealthCheck)

	api := router.Group("/api/v1")
	{
		// Публичные эндпоинты
		api.POST("/accounts", h.CreateAccount)
		api.GET("/accounts/:user_id", h.GetAccount)

		// Защищенные эндпоинты (требуют авторизации)
		auth := api.Group("/billing")
		auth.Use(h.authMiddleware.AuthRequired())
		{
			// Получение информации о своем аккаунте
			auth.GET("/account", h.GetCurrentAccount)

			// Пополнение баланса для своего аккаунта
			auth.POST("/deposit", h.Deposit)
			auth.POST("/withdraw", h.Withdraw)
		}
	}
}

func (h *BillingHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *BillingHandler) CreateAccount(c *gin.Context) {
	var req entity.CreateAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.billingUseCase.CreateAccount(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *BillingHandler) GetAccount(c *gin.Context) {
	userIDStr := c.Param("user_id")
	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID пользователя"})
		return
	}

	resp, err := h.billingUseCase.GetAccount(c.Request.Context(), uint(userID))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *BillingHandler) GetCurrentAccount(c *gin.Context) {
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "невозможно определить пользователя"})
		return
	}

	resp, err := h.billingUseCase.GetAccount(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Deposit пополняет баланс аккаунта
func (h *BillingHandler) Deposit(c *gin.Context) {
	// Получаем ID пользователя из JWT токена
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "невозможно определить пользователя"})
		return
	}

	var req entity.DepositRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := req.Email
	if email == "" {
		email = auth.GetEmail(c)
	}

	resp, err := h.billingUseCase.Deposit(c.Request.Context(), userID, req.Amount, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// Withdraw снимает деньги с аккаунта
func (h *BillingHandler) Withdraw(c *gin.Context) {
	// Получаем ID пользователя из JWT токена
	userID := auth.GetUserID(c)
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "невозможно определить пользователя"})
		return
	}

	var req entity.WithdrawRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	email := req.Email
	if email == "" {
		email = auth.GetEmail(c)
	}

	resp, err := h.billingUseCase.Withdraw(c.Request.Context(), userID, req.Amount, email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if !resp.Success {
		c.JSON(http.StatusBadRequest, gin.H{"error": "недостаточно средств на счете", "transaction": resp.Transaction})
		return
	}

	c.JSON(http.StatusOK, resp)
}
