package usecases

import (
	"context"
	"errors"
	"fmt"
	"time"

	"room-booking/internal/models"
	"room-booking/internal/repository"
)

var (
	ErrSlotInPast      = errors.New("cannot book a slot in the past")
	ErrSlotNotFound    = errors.New("slot not found")
	ErrBookingNotFound = errors.New("booking not found")
	ErrForbidden       = errors.New("forbidden")
)

type BookingUseCase interface {
	CreateBooking(ctx context.Context, userID, slotID string, createConf bool) (*models.Booking, error)
	CancelBooking(ctx context.Context, userID, bookingID string) (*models.Booking, error)
	GetAllBookings(ctx context.Context, page, pageSize int) ([]models.Booking, int, error)
	GetMyBookings(ctx context.Context, userID string) ([]models.Booking, error)
}

type bookingUC struct {
	bookingRepo repository.BookingRepository
	slotRepo    repository.SlotRepository
}

func NewBookingUseCase(bRepo repository.BookingRepository, sRepo repository.SlotRepository) BookingUseCase {
	return &bookingUC{bookingRepo: bRepo, slotRepo: sRepo}
}

// CreateBooking - создание брони
func (uc *bookingUC) CreateBooking(ctx context.Context, userID, slotID string, createConf bool) (*models.Booking, error) {
	// Проверяем, что слот не в прошлом
	slot, err := uc.slotRepo.GetByID(ctx, slotID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrSlotNotFound
		}
		return nil, err
	}
	if slot.StartTime.Before(time.Now().UTC()) {
		return nil, ErrSlotInPast
	}

	// Генерируем ссылку
	var confLink *string
	if createConf {
		link := fmt.Sprintf("https://meet.example.com/%s", slot.ID)
		confLink = &link
	}

	// Создаем бронь
	booking := &models.Booking{
		UserID:         userID,
		SlotID:         slotID,
		Status:         "active",
		ConferenceLink: confLink,
	}

	if err := uc.bookingRepo.Create(ctx, booking); err != nil {
		return nil, err
	}
	return booking, nil
}

// CancelBooking - отмена брони (идемпотентная)
func (uc *bookingUC) CancelBooking(ctx context.Context, userID, bookingID string) (*models.Booking, error) {
	// Находим бронь
	booking, err := uc.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}

	// Проверяем, что юзер отменяет СВОЮ бронь
	if booking.UserID != userID {
		return nil, ErrForbidden
	}

	// Идемпотентность: если уже отменена, просто возвращаем ее
	if booking.Status == "cancelled" {
		return booking, nil
	}

	// Меняем статус
	if err := uc.bookingRepo.UpdateStatus(ctx, bookingID, "cancelled"); err != nil {
		return nil, err
	}
	booking.Status = "cancelled" // Обновляем локальный объект для ответа

	return booking, nil
}

// GetAllBookings - для админа, с пагинацией
func (uc *bookingUC) GetAllBookings(ctx context.Context, page, pageSize int) ([]models.Booking, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	return uc.bookingRepo.GetList(ctx, pageSize, offset)
}

// GetMyBookings - для юзера, только будущие
func (uc *bookingUC) GetMyBookings(ctx context.Context, userID string) ([]models.Booking, error) {
	return uc.bookingRepo.GetMyFuture(ctx, userID, time.Now().UTC())
}
