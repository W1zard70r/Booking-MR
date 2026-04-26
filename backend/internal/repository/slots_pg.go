package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"room-booking/internal/models"
)

type slotPG struct {
	db *sql.DB
}

func NewSlotPG(db *sql.DB) SlotRepository {
	return &slotPG{db: db}
}

// BulkInsert вставляет массив слотов за один запрос.
// Используем синтаксис: INSERT INTO slots (...) VALUES (...), (...), ...
// ON CONFLICT DO NOTHING нужен, чтобы если мы попытаемся сгенерировать слоты
// на день, где они уже есть, база не упала с ошибкой, а просто проигнорировала дубли.
func (r *slotPG) BulkInsert(ctx context.Context, slots []models.Slot) error {
	if len(slots) == 0 {
		return nil
	}

	query := `INSERT INTO slots (room_id, start_time, end_time) VALUES `
	values := make([]interface{}, 0, len(slots)*3)
	placeholders := make([]string, 0, len(slots))

	for i, slot := range slots {
		base := i * 3
		placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d)", base+1, base+2, base+3))
		values = append(values, slot.RoomID, slot.StartTime, slot.EndTime)
	}

	query += strings.Join(placeholders, ", ")
	query += ` ON CONFLICT (room_id, start_time) DO NOTHING`

	_, err := r.db.ExecContext(ctx, query, values...)
	return err
}

// GetAvailable возвращает слоты для переговорки на конкретную дату (UTC),
// на которые НЕТ 'active' брони.
func (r *slotPG) GetAvailable(ctx context.Context, roomID string, date time.Time) ([]models.Slot, error) {
	// слоты за сутки: от date 00:00:00 до date 23:59:59
	startOfDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := startOfDay.Add(24 * time.Hour)

	query := `
		SELECT s.id, s.room_id, s.start_time, s.end_time
		FROM slots s
		LEFT JOIN bookings b ON s.id = b.slot_id AND b.status = 'active'
		WHERE s.room_id = $1 
		  AND s.start_time >= $2 
		  AND s.start_time < $3
		  AND b.id IS NULL -- Если b.id IS NULL, значит активной брони нет
		ORDER BY s.start_time ASC
	`

	rows, err := r.db.QueryContext(ctx, query, roomID, startOfDay, endOfDay)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var slots []models.Slot
	for rows.Next() {
		var s models.Slot
		if err := rows.Scan(&s.ID, &s.RoomID, &s.StartTime, &s.EndTime); err != nil {
			return nil, err
		}
		slots = append(slots, s)
	}

	if slots == nil {
		slots = make([]models.Slot, 0)
	}
	return slots, nil
}

func (r *slotPG) GetByID(ctx context.Context, id string) (*models.Slot, error) {
	var s models.Slot
	query := `SELECT id, room_id, start_time, end_time FROM slots WHERE id = $1`
	err := r.db.QueryRowContext(ctx, query, id).Scan(&s.ID, &s.RoomID, &s.StartTime, &s.EndTime)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return &s, nil
}
