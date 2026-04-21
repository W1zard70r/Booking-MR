package usecases

import (
	"context"
	"errors"
	"room-booking/internal/models"
	"room-booking/internal/repository"
)

var (
	ErrInvalidDayOfWeek = errors.New("day of week must be between 1 and 7")
	ErrInvalidTimeRange = errors.New("invalid time range")
)

type ScheduleUseCase interface {
	CreateSchedule(ctx context.Context, schedule *models.Schedule) error
	SyncAllSlots(ctx context.Context) error
}

type scheduleUC struct {
	scheduleRepo repository.ScheduleRepository
	slotRepo     repository.SlotRepository
}

func NewScheduleUseCase(schedRepo repository.ScheduleRepository, slotRepo repository.SlotRepository) ScheduleUseCase {
	return &scheduleUC{
		scheduleRepo: schedRepo,
		slotRepo:     slotRepo,
	}
}

func (uc *scheduleUC) CreateSchedule(ctx context.Context, schedule *models.Schedule) error {
	if len(schedule.DaysOfWeek) == 0 {
		return ErrInvalidDayOfWeek
	}

	for _, day := range schedule.DaysOfWeek {
		// Вызываем репозиторий с новыми аргументами
		id, err := uc.scheduleRepo.Create(ctx, schedule.RoomID, day, schedule.StartTime, schedule.EndTime)
		if err != nil {
			return err
		}
		schedule.ID = id // Сохраняем последний ID (или можно собирать все)

		// Генерация слотов
		slots, err := GenerateSlots(schedule.RoomID, day, schedule.StartTime, schedule.EndTime, 30)
		if err != nil {
			return ErrInvalidTimeRange
		}

		if len(slots) > 0 {
			if err := uc.slotRepo.BulkInsert(ctx, slots); err != nil {
				return err
			}
		}
	}
	return nil
}

func (uc *scheduleUC) SyncAllSlots(ctx context.Context) error {
	schedules, err := uc.scheduleRepo.GetAll(ctx)
	if err != nil {
		return err
	}

	for _, s := range schedules {
		for _, day := range s.DaysOfWeek {
			// Генерируем слоты на 30 дней вперед от текущего момента
			slots, err := GenerateSlots(s.RoomID, day, s.StartTime, s.EndTime, 30)
			if err != nil {
				continue
			}
			if len(slots) > 0 {
				// BulkInsert проигнорирует те, что уже есть в базе (ON CONFLICT DO NOTHING)
				_ = uc.slotRepo.BulkInsert(ctx, slots)
			}
		}
	}
	return nil
}
