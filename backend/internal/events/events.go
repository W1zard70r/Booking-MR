package events

import (
	"context"
	"time"
)

const (
	BookingCreatedEvent   = "booking.created"
	BookingCancelledEvent = "booking.cancelled"
)

type BookingEvent struct {
	EventType             string     `json:"eventType"`
	BookingID             string     `json:"bookingId"`
	UserID                string     `json:"userId"`
	SlotID                string     `json:"slotId"`
	OccurredAt            time.Time  `json:"occurredAt"`
	RoomID                string     `json:"roomId,omitempty"`
	SlotStart             *time.Time `json:"slotStart,omitempty"`
	SlotEnd               *time.Time `json:"slotEnd,omitempty"`
	ConferenceLinkCreated *bool      `json:"conferenceLinkCreated,omitempty"`
}

type Publisher interface {
	PublishBookingEvent(ctx context.Context, event BookingEvent) error
}

type NoopPublisher struct{}

func (NoopPublisher) PublishBookingEvent(context.Context, BookingEvent) error {
	return nil
}
