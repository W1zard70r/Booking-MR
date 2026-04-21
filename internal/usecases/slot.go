package usecases

import (
	"context"
	"errors"
	"time"

	"room-booking/internal/models"
	"room-booking/internal/repository"
)

var ErrRoomNotFound = errors.New("room not found")

type SlotUseCase interface {
	GetAvailableSlots(ctx context.Context, roomID string, date time.Time) ([]models.Slot, error)
}

type slotUC struct {
	slotRepo repository.SlotRepository
	roomRepo repository.RoomRepository
}

func NewSlotUseCase(slotRepo repository.SlotRepository, roomRepo repository.RoomRepository) SlotUseCase {
	return &slotUC{slotRepo: slotRepo, roomRepo: roomRepo}
}

func (uc *slotUC) GetAvailableSlots(ctx context.Context, roomID string, date time.Time) ([]models.Slot, error) {
	// Проверяем, что такая переговорка вообще существует
	_, err := uc.roomRepo.GetByID(ctx, roomID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrRoomNotFound
		}
		return nil, err
	}

	// Если существует, ищем для нее доступные слоты
	return uc.slotRepo.GetAvailable(ctx, roomID, date)
}
