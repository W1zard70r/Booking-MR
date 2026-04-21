package usecases

import (
	"context"
	"errors"
	"room-booking/internal/models"
	"room-booking/internal/repository"
)

type RoomUseCase interface {
	CreateRoom(ctx context.Context, room *models.Room) error
	GetRooms(ctx context.Context) ([]models.Room, error)
}

type roomUC struct {
	repo repository.RoomRepository
}

func NewRoomUseCase(repo repository.RoomRepository) RoomUseCase {
	return &roomUC{repo: repo}
}

func (uc *roomUC) CreateRoom(ctx context.Context, room *models.Room) error {
	if room.Name == "" {
		return errors.New("room name is required")
	}
	return uc.repo.Create(ctx, room)
}

func (uc *roomUC) GetRooms(ctx context.Context) ([]models.Room, error) {
	return uc.repo.GetAll(ctx)
}
