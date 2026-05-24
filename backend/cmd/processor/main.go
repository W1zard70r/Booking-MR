package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"room-booking/internal/config"
	"room-booking/internal/events"
	"room-booking/internal/logging"

	"github.com/joho/godotenv"
)

const processingDelay = 3 * time.Second

func main() {
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := config.Load()
	logger, err := logging.New(cfg.Logging)
	if err != nil {
		slog.Error("failed to configure logger", "error", err)
		os.Exit(1)
	}
	if cfg.RabbitMQ.URL == "" {
		logger.Error("rabbitmq_url_required")
		os.Exit(1)
	}

	statusPublisher, err := events.NewRabbitMQPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.BookingStatusEventsQueue)
	if err != nil {
		logger.Error("rabbitmq_status_publisher_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := statusPublisher.Close(); err != nil {
			logger.Error("rabbitmq_status_publisher_close_failed", "error", err)
		}
	}()

	closeConsumer, err := events.StartBookingEventConsumer(
		context.Background(),
		cfg.RabbitMQ.URL,
		cfg.RabbitMQ.BookingEventsQueue,
		"booking-processor",
		logger,
		handleBookingCreated(statusPublisher, logger),
	)
	if err != nil {
		logger.Error("rabbitmq_booking_consumer_init_failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := closeConsumer(); err != nil {
			logger.Error("rabbitmq_booking_consumer_close_failed", "error", err)
		}
	}()

	logger.Info(
		"processor_started",
		"booking_queue", cfg.RabbitMQ.BookingEventsQueue,
		"status_queue", cfg.RabbitMQ.BookingStatusEventsQueue,
		"processing_delay", processingDelay.String(),
	)

	select {}
}

func handleBookingCreated(publisher events.Publisher, logger *slog.Logger) events.BookingEventHandler {
	return func(ctx context.Context, event events.BookingEvent) error {
		if event.EventType != events.BookingCreatedEvent {
			return nil
		}

		logger.InfoContext(ctx, "booking_processing_started", "booking_id", event.BookingID)
		time.Sleep(processingDelay)

		statusEvent := events.BookingEvent{
			EventType:      events.BookingStatusChanged,
			BookingID:      event.BookingID,
			UserID:         event.UserID,
			SlotID:         event.SlotID,
			OccurredAt:     time.Now().UTC(),
			PreviousStatus: "processing",
			NewStatus:      "active",
		}
		if err := publisher.PublishBookingEvent(ctx, statusEvent); err != nil {
			return err
		}

		logger.InfoContext(
			ctx,
			"booking_processing_finished",
			"booking_id", event.BookingID,
			"new_status", statusEvent.NewStatus,
		)
		return nil
	}
}
