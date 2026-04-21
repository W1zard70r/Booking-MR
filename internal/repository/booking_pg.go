package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"room-booking/internal/models"

	"github.com/lib/pq"
)

var ErrSlotAlreadyBooked = errors.New("slot is already booked")

type bookingPG struct {
	db *sql.DB
}

func NewBookingPG(db *sql.DB) BookingRepository {
	return &bookingPG{db: db}
}

func (r *bookingPG) Create(ctx context.Context, b *models.Booking) error {
	query := `
		INSERT INTO bookings (user_id, slot_id, status, conference_link) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query,
		b.UserID, b.SlotID, b.Status, b.ConferenceLink,
	).Scan(&b.ID, &b.CreatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return ErrSlotAlreadyBooked
		}
		return err
	}
	return nil
}

func (r *bookingPG) GetByID(ctx context.Context, id string) (*models.Booking, error) {
	var b models.Booking
	query := `SELECT id, user_id, slot_id, status, conference_link, created_at FROM bookings WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&b.ID, &b.UserID, &b.SlotID, &b.Status, &b.ConferenceLink, &b.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &b, nil
}

func (r *bookingPG) UpdateStatus(ctx context.Context, id string, status string) error {
	query := `UPDATE bookings SET status = $1 WHERE id = $2`
	_, err := r.db.ExecContext(ctx, query, status, id)
	return err
}

func (r *bookingPG) GetList(ctx context.Context, limit, offset int) ([]models.Booking, int, error) {
	var total int
	err := r.db.QueryRowContext(ctx, `SELECT count(*) FROM bookings`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	query := `
		SELECT id, user_id, slot_id, status, conference_link, created_at 
		FROM bookings 
		ORDER BY created_at DESC 
		LIMIT $1 OFFSET $2`

	rows, err := r.db.QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.SlotID, &b.Status, &b.ConferenceLink, &b.CreatedAt); err != nil {
			return nil, 0, err
		}
		bookings = append(bookings, b)
	}

	if bookings == nil {
		bookings = make([]models.Booking, 0)
	}

	return bookings, total, nil
}

func (r *bookingPG) GetMyFuture(ctx context.Context, userID string, now time.Time) ([]models.Booking, error) {
	query := `
		SELECT b.id, b.user_id, b.slot_id, b.status, b.conference_link, b.created_at
		FROM bookings b
		JOIN slots s ON b.slot_id = s.id
		WHERE b.user_id = $1 AND s.start_time >= $2
		ORDER BY s.start_time ASC
	`

	rows, err := r.db.QueryContext(ctx, query, userID, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bookings []models.Booking
	for rows.Next() {
		var b models.Booking
		if err := rows.Scan(&b.ID, &b.UserID, &b.SlotID, &b.Status, &b.ConferenceLink, &b.CreatedAt); err != nil {
			return nil, err
		}
		bookings = append(bookings, b)
	}

	if bookings == nil {
		bookings = make([]models.Booking, 0)
	}

	return bookings, nil
}
