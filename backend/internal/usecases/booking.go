package usecases

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	logger      *slog.Logger
}

func NewBookingUseCase(bRepo repository.BookingRepository, sRepo repository.SlotRepository, logger ...*slog.Logger) BookingUseCase {
	return &bookingUC{
		bookingRepo: bRepo,
		slotRepo:    sRepo,
		logger:      resolveLogger(logger),
	}
}

func (uc *bookingUC) CreateBooking(ctx context.Context, userID, slotID string, createConf bool) (*models.Booking, error) {
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

	var confLink *string
	if createConf {
		link := fmt.Sprintf("https://meet.example.com/%s", slot.ID)
		confLink = &link
	}

	booking := &models.Booking{
		UserID:         userID,
		SlotID:         slotID,
		Status:         "active",
		ConferenceLink: confLink,
	}

	if err := uc.bookingRepo.Create(ctx, booking); err != nil {
		return nil, err
	}

	uc.logger.InfoContext(
		ctx,
		"booking_created",
		"booking_id", booking.ID,
		"user_id", booking.UserID,
		"slot_id", booking.SlotID,
		"room_id", slot.RoomID,
		"slot_start", slot.StartTime,
		"slot_end", slot.EndTime,
		"conference_link_requested", createConf,
		"conference_link_created", booking.ConferenceLink != nil,
	)

	return booking, nil
}

func (uc *bookingUC) CancelBooking(ctx context.Context, userID, bookingID string) (*models.Booking, error) {
	booking, err := uc.bookingRepo.GetByID(ctx, bookingID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrBookingNotFound
		}
		return nil, err
	}

	if booking.UserID != userID {
		return nil, ErrForbidden
	}

	if booking.Status == "cancelled" {
		return booking, nil
	}

	if err := uc.bookingRepo.UpdateStatus(ctx, bookingID, "cancelled"); err != nil {
		return nil, err
	}
	booking.Status = "cancelled"

	uc.logger.InfoContext(
		ctx,
		"booking_cancelled",
		"booking_id", booking.ID,
		"user_id", booking.UserID,
		"slot_id", booking.SlotID,
	)

	return booking, nil
}

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

func (uc *bookingUC) GetMyBookings(ctx context.Context, userID string) ([]models.Booking, error) {
	return uc.bookingRepo.GetMyFuture(ctx, userID, time.Now().UTC())
}
