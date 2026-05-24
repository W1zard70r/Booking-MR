package events

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQPublisher struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

func NewRabbitMQPublisher(url string, queue string) (*RabbitMQPublisher, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if _, err := declareBookingEventsQueue(channel, queue); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}

	return &RabbitMQPublisher{
		conn:    conn,
		channel: channel,
		queue:   queue,
	}, nil
}

func (p *RabbitMQPublisher) PublishBookingEvent(ctx context.Context, event BookingEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}

	return p.channel.PublishWithContext(ctx, "", p.queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		Body:         payload,
	})
}

func (p *RabbitMQPublisher) Close() error {
	if err := p.channel.Close(); err != nil {
		_ = p.conn.Close()
		return err
	}
	return p.conn.Close()
}

func StartBookingEventConsumer(ctx context.Context, url string, queue string, logger *slog.Logger) (func() error, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	channel, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if _, err := declareBookingEventsQueue(channel, queue); err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}

	deliveries, err := channel.Consume(queue, "booking-backend-logger", true, false, false, false, nil)
	if err != nil {
		_ = channel.Close()
		_ = conn.Close()
		return nil, err
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case delivery, ok := <-deliveries:
				if !ok {
					return
				}

				var event BookingEvent
				if err := json.Unmarshal(delivery.Body, &event); err != nil {
					logger.ErrorContext(ctx, "booking_event_decode_failed", "error", err)
					continue
				}

				logger.InfoContext(
					ctx,
					"booking_event_consumed",
					"event_type", event.EventType,
					"booking_id", event.BookingID,
					"user_id", event.UserID,
					"slot_id", event.SlotID,
				)
			}
		}
	}()

	return func() error {
		if err := channel.Close(); err != nil {
			_ = conn.Close()
			return err
		}
		return conn.Close()
	}, nil
}

func declareBookingEventsQueue(channel *amqp.Channel, queue string) (amqp.Queue, error) {
	return channel.QueueDeclare(queue, true, false, false, false, nil)
}
