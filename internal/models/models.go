package models

import "time"

// User представляет пользователя системы (admin или user)
type User struct {
	ID           string    `json:"id" db:"id"`
	Email        string    `json:"email" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	CreatedAt    time.Time `json:"createdAt" db:"created_at"`
}

// Room - переговорка
type Room struct {
	ID          string    `json:"id" db:"id"`
	Name        string    `json:"name" db:"name"`
	Description *string   `json:"description" db:"description"`
	Capacity    *int      `json:"capacity" db:"capacity"`
	CreatedAt   time.Time `json:"createdAt" db:"created_at"`
}

// Schedule - расписание для переговорки
type Schedule struct {
	ID         string `json:"id" db:"id"`
	RoomID     string `json:"roomId" db:"room_id"`
	DaysOfWeek []int  `json:"daysOfWeek" db:"-"`
	StartTime  string `json:"startTime" db:"start_time"`
	EndTime    string `json:"endTime" db:"end_time"`
}

// Slot - конкретный 30-минутный интервал
type Slot struct {
	ID        string    `json:"id" db:"id"`
	RoomID    string    `json:"roomId" db:"room_id"`
	StartTime time.Time `json:"start" db:"start_time"` // В UTC!
	EndTime   time.Time `json:"end" db:"end_time"`     // В UTC!
}

// Booking - бронь
type Booking struct {
	ID             string    `json:"id" db:"id"`
	SlotID         string    `json:"slotId" db:"slot_id"`
	UserID         string    `json:"userId" db:"user_id"`
	Status         string    `json:"status" db:"status"` // active, cancelled
	ConferenceLink *string   `json:"conferenceLink,omitempty" db:"conference_link"`
	CreatedAt      time.Time `json:"createdAt" db:"created_at"`
}
