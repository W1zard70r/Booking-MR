package usecases

import (
	"context"
	"errors"
	"testing"
	"time"

	"room-booking/internal/events"
	"room-booking/internal/models"
)

type fakeBookingRepo struct {
	booking   *models.Booking
	createErr error
}

func (r *fakeBookingRepo) Create(ctx context.Context, booking *models.Booking) error {
	if r.createErr != nil {
		return r.createErr
	}
	booking.ID = "booking-1"
	booking.CreatedAt = time.Now().UTC()
	r.booking = booking
	return nil
}

func (r *fakeBookingRepo) GetByID(ctx context.Context, id string) (*models.Booking, error) {
	if r.booking == nil || r.booking.ID != id {
		return nil, errors.New("not found")
	}
	return r.booking, nil
}

func (r *fakeBookingRepo) UpdateStatus(ctx context.Context, id string, status string) error {
	r.booking.Status = status
	return nil
}

func (r *fakeBookingRepo) GetList(ctx context.Context, limit, offset int) ([]models.Booking, int, error) {
	return nil, 0, nil
}

func (r *fakeBookingRepo) GetMyFuture(ctx context.Context, userID string, now time.Time) ([]models.Booking, error) {
	return nil, nil
}

type fakeSlotRepo struct {
	slot *models.Slot
	err  error
}

func (r *fakeSlotRepo) BulkInsert(ctx context.Context, slots []models.Slot) error {
	return nil
}

func (r *fakeSlotRepo) GetAvailable(ctx context.Context, roomID string, date time.Time) ([]models.Slot, error) {
	return nil, nil
}

func (r *fakeSlotRepo) GetByID(ctx context.Context, id string) (*models.Slot, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.slot, nil
}

type capturePublisher struct {
	events []events.BookingEvent
}

func (p *capturePublisher) PublishBookingEvent(ctx context.Context, event events.BookingEvent) error {
	p.events = append(p.events, event)
	return nil
}

func TestCreateBookingPublishesCreatedEvent(t *testing.T) {
	publisher := &capturePublisher{}
	uc := NewBookingUseCase(
		&fakeBookingRepo{},
		&fakeSlotRepo{slot: futureSlot()},
		publisher,
	)

	booking, err := uc.CreateBooking(context.Background(), "user-1", "slot-1", true)
	if err != nil {
		t.Fatalf("CreateBooking failed: %v", err)
	}
	if booking.ID == "" {
		t.Fatal("expected booking ID to be set")
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(publisher.events))
	}

	event := publisher.events[0]
	if event.EventType != events.BookingCreatedEvent {
		t.Fatalf("expected %s, got %s", events.BookingCreatedEvent, event.EventType)
	}
	if event.BookingID != booking.ID || event.UserID != "user-1" || event.SlotID != "slot-1" {
		t.Fatalf("unexpected event payload: %+v", event)
	}
	if event.RoomID != "room-1" || event.SlotStart == nil || event.SlotEnd == nil {
		t.Fatalf("expected slot details in event: %+v", event)
	}
	if event.ConferenceLinkCreated == nil || !*event.ConferenceLinkCreated {
		t.Fatalf("expected conferenceLinkCreated=true: %+v", event)
	}
}

func TestCreateBookingDoesNotPublishOnCreateError(t *testing.T) {
	publisher := &capturePublisher{}
	uc := NewBookingUseCase(
		&fakeBookingRepo{createErr: errors.New("create failed")},
		&fakeSlotRepo{slot: futureSlot()},
		publisher,
	)

	if _, err := uc.CreateBooking(context.Background(), "user-1", "slot-1", false); err == nil {
		t.Fatal("expected create error")
	}
	if len(publisher.events) != 0 {
		t.Fatalf("expected no events, got %d", len(publisher.events))
	}
}

func TestCancelBookingPublishesCancelledEvent(t *testing.T) {
	publisher := &capturePublisher{}
	bookingRepo := &fakeBookingRepo{booking: &models.Booking{
		ID:     "booking-1",
		UserID: "user-1",
		SlotID: "slot-1",
		Status: "active",
	}}
	uc := NewBookingUseCase(bookingRepo, &fakeSlotRepo{slot: futureSlot()}, publisher)

	booking, err := uc.CancelBooking(context.Background(), "user-1", "booking-1")
	if err != nil {
		t.Fatalf("CancelBooking failed: %v", err)
	}
	if booking.Status != "cancelled" {
		t.Fatalf("expected cancelled status, got %s", booking.Status)
	}
	if len(publisher.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(publisher.events))
	}
	if publisher.events[0].EventType != events.BookingCancelledEvent {
		t.Fatalf("expected %s, got %s", events.BookingCancelledEvent, publisher.events[0].EventType)
	}
}

func TestCancelBookingDoesNotPublishWhenAlreadyCancelled(t *testing.T) {
	publisher := &capturePublisher{}
	bookingRepo := &fakeBookingRepo{booking: &models.Booking{
		ID:     "booking-1",
		UserID: "user-1",
		SlotID: "slot-1",
		Status: "cancelled",
	}}
	uc := NewBookingUseCase(bookingRepo, &fakeSlotRepo{slot: futureSlot()}, publisher)

	if _, err := uc.CancelBooking(context.Background(), "user-1", "booking-1"); err != nil {
		t.Fatalf("CancelBooking failed: %v", err)
	}
	if len(publisher.events) != 0 {
		t.Fatalf("expected no events, got %d", len(publisher.events))
	}
}

func futureSlot() *models.Slot {
	start := time.Now().UTC().Add(time.Hour)
	return &models.Slot{
		ID:        "slot-1",
		RoomID:    "room-1",
		StartTime: start,
		EndTime:   start.Add(30 * time.Minute),
	}
}
