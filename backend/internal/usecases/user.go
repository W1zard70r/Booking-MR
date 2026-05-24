package usecases

import (
	"context"
	"errors"
	"log/slog"

	"room-booking/internal/metrics"
	"room-booking/internal/models"
	"room-booking/internal/repository"
	"room-booking/pkg/auth"

	"golang.org/x/crypto/bcrypt"
)

type UserUseCase interface {
	Register(ctx context.Context, email, password, role string) (*models.User, error)
	Login(ctx context.Context, email, password string) (string, error)
}

type userUC struct {
	repo      repository.UserRepository
	jwtSecret string
	logger    *slog.Logger
}

func NewUserUseCase(repo repository.UserRepository, secret string, logger ...*slog.Logger) UserUseCase {
	return &userUC{repo: repo, jwtSecret: secret, logger: resolveLogger(logger)}
}

func (uc *userUC) Register(ctx context.Context, email, password, role string) (*models.User, error) {
	hashedBytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &models.User{
		Email:        email,
		PasswordHash: string(hashedBytes),
		Role:         role,
	}

	if err := uc.repo.Create(ctx, user); err != nil {
		return nil, err
	}

	uc.logger.InfoContext(
		ctx,
		"user_registered",
		"user_id", user.ID,
		"email", user.Email,
		"role", user.Role,
	)
	metrics.RecordBusinessEvent("user_registered")

	return user, nil
}

func (uc *userUC) Login(ctx context.Context, email, password string) (string, error) {
	user, err := uc.repo.GetByEmail(ctx, email)
	if err != nil {
		return "", errors.New("invalid email or password")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("invalid email or password")
	}

	token, err := auth.GenerateToken(user.ID, user.Role, uc.jwtSecret)
	if err != nil {
		return "", err
	}

	uc.logger.InfoContext(
		ctx,
		"user_logged_in",
		"user_id", user.ID,
		"role", user.Role,
	)
	metrics.RecordBusinessEvent("user_logged_in")

	return token, nil
}
