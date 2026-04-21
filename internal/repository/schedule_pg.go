package repository

import (
	"context"
	"database/sql"
	"errors"
	"room-booking/internal/models"

	"github.com/lib/pq"
)

var ErrScheduleExists = errors.New("schedule already exists")

type schedulePG struct {
	db *sql.DB
}

func NewSchedulePG(db *sql.DB) ScheduleRepository {
	return &schedulePG{db: db}
}

func (r *schedulePG) Create(ctx context.Context, roomID string, day int, start, end string) (string, error) {
	query := `
		INSERT INTO schedules (room_id, day_of_week, start_time, end_time) 
		VALUES ($1, $2, $3, $4) 
		RETURNING id`

	var id string
	err := r.db.QueryRowContext(ctx, query, roomID, day, start, end).Scan(&id)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return "", ErrAlreadyExists
		}
		return "", err
	}
	return id, nil
}

func (r *schedulePG) GetByRoomID(ctx context.Context, roomID string) ([]models.Schedule, error) {
	query := `SELECT id, room_id, day_of_week, start_time, end_time FROM schedules WHERE room_id = $1`
	rows, err := r.db.QueryContext(ctx, query, roomID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var day int

		if err := rows.Scan(&s.ID, &s.RoomID, &day, &s.StartTime, &s.EndTime); err != nil {
			return nil, err
		}

		s.DaysOfWeek = []int{day}
		schedules = append(schedules, s)
	}
	return schedules, nil
}

func (r *schedulePG) GetAll(ctx context.Context) ([]models.Schedule, error) {
	query := `SELECT id, room_id, day_of_week, start_time, end_time FROM schedules`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schedules []models.Schedule
	for rows.Next() {
		var s models.Schedule
		var day int
		if err := rows.Scan(&s.ID, &s.RoomID, &day, &s.StartTime, &s.EndTime); err != nil {
			return nil, err
		}
		s.DaysOfWeek = []int{day}
		schedules = append(schedules, s)
	}
	return schedules, nil
}
