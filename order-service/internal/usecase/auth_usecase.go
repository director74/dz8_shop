package usecase

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/director74/dz8_shop/order-service/internal/entity"
	"github.com/director74/dz8_shop/order-service/internal/repo"
	"github.com/director74/dz8_shop/pkg/auth"
)

// ErrInvalidCredentials ошибка при неверных учетных данных
var ErrInvalidCredentials = errors.New("неверные учетные данные")

// ErrUserAlreadyExists ошибка, когда пользователь уже существует
var ErrUserAlreadyExists = errors.New("пользователь с таким email или username уже существует")

// AuthUseCase сервис аутентификации
type AuthUseCase struct {
	userRepo   repo.UserRepository
	jwtManager *auth.JWTManager
	billing    BillingService
}

func NewAuthUseCase(userRepo repo.UserRepository, jwtManager *auth.JWTManager, billing BillingService) *AuthUseCase {
	return &AuthUseCase{
		userRepo:   userRepo,
		jwtManager: jwtManager,
		billing:    billing,
	}
}

func (uc *AuthUseCase) Register(ctx context.Context, req entity.RegisterRequest) (*entity.RegisterResponse, error) {
	existingUser, err := uc.userRepo.GetByEmail(ctx, req.Email)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	existingUser, err = uc.userRepo.GetByUsername(ctx, req.Username)
	if err == nil && existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	// Хешируем пароль
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Username:  req.Username,
		Email:     req.Email,
		Password:  hashedPassword,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := uc.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	if err := uc.billing.CreateAccount(ctx, user.ID); err != nil {
		// При ошибке создания аккаунта в биллинге удаляем пользователя
		deleteErr := uc.userRepo.Delete(ctx, user.ID)
		if deleteErr != nil {
			// Логируем ошибку удаления, но возвращаем основную ошибку
			fmt.Printf("Ошибка при удалении пользователя после неудачного создания аккаунта в биллинге: %v\n", deleteErr)
		}
		return nil, fmt.Errorf("ошибка при создании аккаунта в биллинге: %w", err)
	}

	return &entity.RegisterResponse{
		ID:        user.ID,
		Username:  user.Username,
		Email:     user.Email,
		CreatedAt: user.CreatedAt,
	}, nil
}

// Login аутентифицирует пользователя и возвращает JWT токен
func (uc *AuthUseCase) Login(ctx context.Context, req entity.LoginRequest) (*entity.LoginResponse, error) {
	// Ищем пользователя по username
	user, err := uc.userRepo.GetByUsername(ctx, req.Username)
	if err != nil {
		if errors.Is(err, repo.ErrUserNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if !auth.CheckPasswordHash(req.Password, user.Password) {
		return nil, ErrInvalidCredentials
	}

	// Генерируем JWT токен
	token, err := uc.jwtManager.GenerateToken(user.ID, user.Username, user.Email)
	if err != nil {
		return nil, err
	}

	return &entity.LoginResponse{
		ID:       user.ID,
		Username: user.Username,
		Email:    user.Email,
		Token:    token,
	}, nil
}
