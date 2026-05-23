package usecases

import (
	"context"
	"errors"
	"log/slog"

	"room-booking/internal/models"
	"room-booking/internal/repository"
)

type RoomUseCase interface {
	CreateRoom(ctx context.Context, room *models.Room) error
	GetRooms(ctx context.Context) ([]models.Room, error)
}

type roomUC struct {
	repo   repository.RoomRepository
	logger *slog.Logger
}

func NewRoomUseCase(repo repository.RoomRepository, logger ...*slog.Logger) RoomUseCase {
	return &roomUC{repo: repo, logger: resolveLogger(logger)}
}

func (uc *roomUC) CreateRoom(ctx context.Context, room *models.Room) error {
	if room.Name == "" {
		return errors.New("room name is required")
	}
	if err := uc.repo.Create(ctx, room); err != nil {
		return err
	}

	uc.logger.InfoContext(
		ctx,
		"room_created",
		"room_id", room.ID,
		"name", room.Name,
		"capacity", room.Capacity,
	)

	return nil
}

func (uc *roomUC) GetRooms(ctx context.Context) ([]models.Room, error) {
	return uc.repo.GetAll(ctx)
}
