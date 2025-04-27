package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/director74/dz8_shop/notification-service/internal/entity"
	"github.com/director74/dz8_shop/notification-service/internal/usecase"
)

type NotificationHandler struct {
	notificationUseCase *usecase.NotificationUseCase
}

func NewNotificationHandler(notificationUseCase *usecase.NotificationUseCase) *NotificationHandler {
	return &NotificationHandler{
		notificationUseCase: notificationUseCase,
	}
}

func (h *NotificationHandler) RegisterRoutes(router *gin.Engine) {
	router.GET("/health", h.HealthCheck)

	api := router.Group("/api/v1")
	{
		api.POST("/notifications", h.SendNotification)
		api.GET("/notifications/:id", h.GetNotification)
		api.GET("/users/:id/notifications", h.ListUserNotifications)
		api.GET("/notifications", h.ListAllNotifications)
	}
}

func (h *NotificationHandler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (h *NotificationHandler) SendNotification(c *gin.Context) {
	var req entity.SendNotificationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := h.notificationUseCase.SendNotification(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, resp)
}

func (h *NotificationHandler) GetNotification(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID"})
		return
	}

	resp, err := h.notificationUseCase.GetNotification(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *NotificationHandler) ListUserNotifications(c *gin.Context) {
	idStr := c.Param("id")
	userID, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "некорректный ID пользователя"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.notificationUseCase.ListUserNotifications(c.Request.Context(), uint(userID), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *NotificationHandler) ListAllNotifications(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	resp, err := h.notificationUseCase.ListAllNotifications(c.Request.Context(), limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
