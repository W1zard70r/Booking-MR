package repository

import (
	"context"
	"database/sql"
	"errors"
	"room-booking/internal/models"
)

type roomPG struct {
	db *sql.DB
}

func NewRoomPG(db *sql.DB) RoomRepository {
	return &roomPG{db: db}
}

func (r *roomPG) Create(ctx context.Context, room *models.Room) error {
	query := `
		INSERT INTO rooms (name, description, capacity) 
		VALUES ($1, $2, $3) 
		RETURNING id, created_at`

	return r.db.QueryRowContext(ctx, query, room.Name, room.Description, room.Capacity).
		Scan(&room.ID, &room.CreatedAt)
}

func (r *roomPG) GetAll(ctx context.Context) ([]models.Room, error) {
	query := `SELECT id, name, description, capacity, created_at FROM rooms ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var rooms []models.Room
	for rows.Next() {
		var room models.Room
		if err := rows.Scan(&room.ID, &room.Name, &room.Description, &room.Capacity, &room.CreatedAt); err != nil {
			return nil, err
		}
		rooms = append(rooms, room)
	}

	if rooms == nil {
		rooms = make([]models.Room, 0)
	}
	return rooms, nil
}

func (r *roomPG) GetByID(ctx context.Context, id string) (*models.Room, error) {
	var room models.Room
	query := `SELECT id, name, description, capacity, created_at FROM rooms WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&room.ID, &room.Name, &room.Description, &room.Capacity, &room.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &room, nil
}
