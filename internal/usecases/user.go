package usecases

import (
	"context"
	"errors"
	"room-booking/internal/models"
	"room-booking/internal/repository"
	"room-booking/pkg/auth"

	"golang.org/x/crypto/bcrypt"
)

type UserUseCase interface {
	Register(ctx context.Context, email, password, role string) (*models.User, error)
	Login(ctx context.Context, email, password string) (string, error) // Возвращает JWT
}

type userUC struct {
	repo      repository.UserRepository
	jwtSecret string
}

func NewUserUseCase(repo repository.UserRepository, secret string) UserUseCase {
	return &userUC{repo: repo, jwtSecret: secret}
}

func (uc *userUC) Register(ctx context.Context, email, password, role string) (*models.User, error) {
	// Хешируем пароль
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedBytes),
		Role:         role,
	}

	// Сохраняем
	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}
	return user, nil
}

func (uc *userUC) Login(ctx context.Context, email, password string) (string, error) {
	// Ищем юзера
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid email or password")
	}

	// Проверяем пароль
	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password))
	if err != nil {
		return "", errors.New("invalid email or password")
	}

	// Генерируем токен
	return auth.GenerateToken(user.ID, user.Role, uc.jwtSecret)
}
