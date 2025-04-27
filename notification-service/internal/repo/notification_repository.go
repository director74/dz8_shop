package repo

import (
	"context"

	"gorm.io/gorm"

	"github.com/director74/dz8_shop/notification-service/internal/entity"
)

// NotificationRepository доступ к хранилищу уведомлений
type NotificationRepository struct {
	db *gorm.DB
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{
		db: db,
	}
}

func (r *NotificationRepository) CreateNotification(ctx context.Context, notification entity.Notification) (entity.Notification, error) {
	err := r.db.WithContext(ctx).Create(&notification).Error
	return notification, err
}

func (r *NotificationRepository) GetNotificationByID(ctx context.Context, id uint) (entity.Notification, error) {
	var notification entity.Notification
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&notification).Error
	return notification, err
}

func (r *NotificationRepository) UpdateNotificationStatus(ctx context.Context, id uint, status string) error {
	return r.db.WithContext(ctx).Model(&entity.Notification{}).Where("id = ?", id).
		Update("status", status).Error
}

func (r *NotificationRepository) ListNotificationsByUserID(ctx context.Context, userID uint, limit, offset int) ([]entity.Notification, int64, error) {
	var notifications []entity.Notification
	var total int64

	r.db.WithContext(ctx).Model(&entity.Notification{}).Where("user_id = ?", userID).Count(&total)
	err := r.db.WithContext(ctx).Where("user_id = ?", userID).Limit(limit).Offset(offset).Order("created_at DESC").Find(&notifications).Error

	return notifications, total, err
}

func (r *NotificationRepository) ListAllNotifications(ctx context.Context, limit, offset int) ([]entity.Notification, int64, error) {
	var notifications []entity.Notification
	var total int64

	r.db.WithContext(ctx).Model(&entity.Notification{}).Count(&total)
	err := r.db.WithContext(ctx).Limit(limit).Offset(offset).Order("created_at DESC").Find(&notifications).Error

	return notifications, total, err
}
