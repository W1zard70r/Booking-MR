package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"room-booking/internal/models"
)

// Ошибки уровня БД
var (
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
)

// Repository объединяет все интерфейсы работы с БД
type Repository struct {
	Room     RoomRepository
	Schedule ScheduleRepository
	Slot     SlotRepository
	Booking  BookingRepository
	User     UserRepository
	Test     TestRepository
}

// New создает новый экземпляр Repository
func New(db *sql.DB) *Repository {
	return &Repository{
		Room:     NewRoomPG(db),
		Schedule: NewSchedulePG(db),
		Slot:     NewSlotPG(db),
		Booking:  NewBookingPG(db),
		User:     NewUserPG(db),
		Test:     NewTestPG(db),
	}
}

// --- Интерфейсы ---

type RoomRepository interface {
	Create(ctx context.Context, room *models.Room) error
	GetAll(ctx context.Context) ([]models.Room, error)
	GetByID(ctx context.Context, id string) (*models.Room, error)
}

type ScheduleRepository interface {
	Create(ctx context.Context, roomID string, day int, start, end string) (string, error)
	GetByRoomID(ctx context.Context, roomID string) ([]models.Schedule, error)
	GetAll(ctx context.Context) ([]models.Schedule, error)
}

type SlotRepository interface {
	// BulkInsert вставляет сразу много слотов (на 30 дней вперед)
	BulkInsert(ctx context.Context, slots []models.Slot) error
	// GetAvailable возвращает слоты, на которые НЕТ активной брони
	GetAvailable(ctx context.Context, roomID string, date time.Time) ([]models.Slot, error)
	GetByID(ctx context.Context, id string) (*models.Slot, error)
}

type BookingRepository interface {
	Create(ctx context.Context, booking *models.Booking) error
	GetByID(ctx context.Context, id string) (*models.Booking, error)
	UpdateStatus(ctx context.Context, id string, status string) error
	// GetList - для админа (с пагинацией)
	GetList(ctx context.Context, limit, offset int) ([]models.Booking, int, error)
	// GetMyFuture - для юзера (только его и только будущие)
	GetMyFuture(ctx context.Context, userID string, now time.Time) ([]models.Booking, error)
}

type TestRepository interface {
	SaveMessage(ctx context.Context, msg string) error
}
