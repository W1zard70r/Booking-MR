package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"room-booking/internal/api"
	"room-booking/internal/config"
	"room-booking/internal/events"
	"room-booking/internal/logging"
	"room-booking/internal/models"
	"room-booking/internal/notifications"
	"room-booking/internal/repository"
	"room-booking/internal/usecases"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	_ = godotenv.Load("../.env")
	_ = godotenv.Load(".env")

	cfg := config.Load()
	logger, err := logging.New(cfg.Logging)
	if err != nil {
		slog.Error("failed to configure logger", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := sql.Open("postgres", cfg.Database.DSN())
	if err != nil {
		logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	pingCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	if err := runStartupMigrations(ctx, db, logger); err != nil {
		logger.Error("startup_migrations_failed", "error", err)
		os.Exit(1)
	}

	repo := repository.New(db)
	emailSender := notifications.NewEmailSender(cfg.SMTP)
	if cfg.SMTP.Enabled {
		logger.Info("smtp_notifications_enabled", "host", cfg.SMTP.Host, "port", cfg.SMTP.Port, "from", cfg.SMTP.From)
	} else {
		logger.Info("smtp_notifications_disabled", "reason", "SMTP_ENABLED is false")
	}

	eventPublisher := events.Publisher(events.NoopPublisher{})
	if cfg.RabbitMQ.URL != "" {
		rabbitPublisher, err := events.NewRabbitMQPublisher(cfg.RabbitMQ.URL, cfg.RabbitMQ.BookingEventsQueue)
		if err != nil {
			logger.Error("rabbitmq_publisher_init_failed", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := rabbitPublisher.Close(); err != nil {
				logger.Error("rabbitmq_publisher_close_failed", "error", err)
			}
		}()
		eventPublisher = rabbitPublisher

		closeConsumer, err := events.StartBookingEventConsumer(
			ctx,
			cfg.RabbitMQ.URL,
			cfg.RabbitMQ.BookingStatusEventsQueue,
			"booking-backend-status-updater",
			logger,
			handleBookingStatusEvent(repo.Booking, repo.User, repo.Room, emailSender, logger),
		)
		if err != nil {
			logger.Error("rabbitmq_consumer_init_failed", "error", err)
			os.Exit(1)
		}
		defer func() {
			if err := closeConsumer(); err != nil {
				logger.Error("rabbitmq_consumer_close_failed", "error", err)
			}
		}()
		logger.Info("rabbitmq_enabled", "booking_queue", cfg.RabbitMQ.BookingEventsQueue, "status_queue", cfg.RabbitMQ.BookingStatusEventsQueue)
	} else {
		logger.Info("rabbitmq_disabled", "reason", "RABBITMQ_URL is empty")
	}

	roomUC := usecases.NewRoomUseCase(repo.Room, logger)
	scheduleUC := usecases.NewScheduleUseCase(repo.Schedule, repo.Slot, logger)
	slotUC := usecases.NewSlotUseCase(repo.Slot, repo.Room)
	bookingUC := usecases.NewBookingUseCase(repo.Booking, repo.Slot, eventPublisher, logger)
	userUC := usecases.NewUserUseCase(repo.User, cfg.JWT.Secret, logger)
	testUC := usecases.NewTestUseCase(repo.Test)

	appAPI := &api.API{
		RoomUC:     roomUC,
		ScheduleUC: scheduleUC,
		SlotUC:     slotUC,
		BookingUC:  bookingUC,
		JWTSecret:  cfg.JWT.Secret,
		UserUC:     userUC,
		TestUC:     testUC,
	}

	router := newRouter(appAPI, cfg.JWT.Secret, logger)
	startSlotSync(ctx, scheduleUC, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info(
		"server_started",
		"port", cfg.Server.Port,
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
	)

	serverErr := make(chan error, 1)
	go func() {
		err := server.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	serverStopped := false
	select {
	case err := <-serverErr:
		serverStopped = true
		if err != nil {
			logger.Error("server_stopped_unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutdown_signal_received")
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("server_shutdown_failed", "error", err)
		os.Exit(1)
	}
	if !serverStopped {
		if err := <-serverErr; err != nil {
			logger.Error("server_stopped_unexpectedly", "error", err)
			os.Exit(1)
		}
	}
	logger.Info("server_shutdown_completed")
}

func runStartupMigrations(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	exists, err := tableExists(ctx, db, "users")
	if err != nil {
		return err
	}
	if exists {
		logger.DebugContext(ctx, "startup_migrations_skipped", "reason", "schema_exists")
		return ensureBookingProcessingSchema(ctx, db, logger)
	}

	migrationPath := filepath.Join("migrations", "000001_init.up.sql")
	query, err := os.ReadFile(migrationPath)
	if err != nil {
		return err
	}
	if _, err := db.ExecContext(ctx, string(query)); err != nil {
		return err
	}

	logger.InfoContext(ctx, "startup_migrations_applied", "migration", migrationPath)
	return ensureBookingProcessingSchema(ctx, db, logger)
}

func ensureBookingProcessingSchema(ctx context.Context, db *sql.DB, logger *slog.Logger) error {
	statements := []string{
		`ALTER TABLE bookings DROP CONSTRAINT IF EXISTS bookings_status_check`,
		`ALTER TABLE bookings ADD CONSTRAINT bookings_status_check CHECK (status IN ('processing', 'active', 'cancelled'))`,
		`DROP INDEX IF EXISTS idx_unique_active_booking`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_unique_active_booking ON bookings (slot_id) WHERE status IN ('processing', 'active')`,
	}

	for _, statement := range statements {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}

	logger.DebugContext(ctx, "booking_processing_schema_ensured")
	return nil
}

func tableExists(ctx context.Context, db *sql.DB, tableName string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(
		ctx,
		`SELECT EXISTS (
			SELECT 1
			FROM information_schema.tables
			WHERE table_schema = 'public' AND table_name = $1
		)`,
		tableName,
	).Scan(&exists)
	return exists, err
}

func handleBookingStatusEvent(
	bookingRepo repository.BookingRepository,
	userRepo repository.UserRepository,
	roomRepo repository.RoomRepository,
	emailSender notifications.EmailSender,
	logger *slog.Logger,
) events.BookingEventHandler {
	return func(ctx context.Context, event events.BookingEvent) error {
		if event.EventType != events.BookingStatusChanged {
			return nil
		}
		if event.NewStatus == "" {
			logger.WarnContext(ctx, "booking_status_event_ignored", "reason", "empty_new_status", "booking_id", event.BookingID)
			return nil
		}

		currentStatus := event.PreviousStatus
		if currentStatus == "" {
			currentStatus = "processing"
		}

		updated, err := bookingRepo.UpdateStatusIfCurrent(ctx, event.BookingID, currentStatus, event.NewStatus)
		if err != nil {
			return err
		}
		if !updated {
			logger.InfoContext(
				ctx,
				"booking_status_update_skipped",
				"booking_id", event.BookingID,
				"expected_status", currentStatus,
				"new_status", event.NewStatus,
			)
			return nil
		}

		logger.InfoContext(
			ctx,
			"booking_status_updated",
			"booking_id", event.BookingID,
			"previous_status", currentStatus,
			"new_status", event.NewStatus,
		)
		if event.NewStatus == "active" {
			sendBookingConfirmedEmail(ctx, event, bookingRepo, userRepo, roomRepo, emailSender, logger)
		}
		return nil
	}
}

func sendBookingConfirmedEmail(
	ctx context.Context,
	event events.BookingEvent,
	bookingRepo repository.BookingRepository,
	userRepo repository.UserRepository,
	roomRepo repository.RoomRepository,
	emailSender notifications.EmailSender,
	logger *slog.Logger,
) {
	user, err := userRepo.GetByID(ctx, event.UserID)
	if err != nil {
		logger.ErrorContext(ctx, "booking_confirmation_email_user_lookup_failed", "booking_id", event.BookingID, "user_id", event.UserID, "error", err)
		return
	}

	booking, err := bookingRepo.GetByID(ctx, event.BookingID)
	if err != nil {
		logger.ErrorContext(ctx, "booking_confirmation_email_booking_lookup_failed", "booking_id", event.BookingID, "error", err)
		return
	}

	roomName := event.RoomID
	if event.RoomID != "" {
		room, err := roomRepo.GetByID(ctx, event.RoomID)
		if err != nil {
			logger.ErrorContext(ctx, "booking_confirmation_email_room_lookup_failed", "booking_id", event.BookingID, "room_id", event.RoomID, "error", err)
		} else {
			roomName = room.Name
		}
	}
	if roomName == "" {
		roomName = "переговорка"
	}

	emailCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	message := notifications.EmailMessage{
		To:      user.Email,
		Subject: "Бронирование переговорки подтверждено",
		Body:    buildBookingConfirmedEmailBody(event, booking, roomName),
	}
	if err := emailSender.Send(emailCtx, message); err != nil {
		logger.ErrorContext(ctx, "booking_confirmation_email_send_failed", "booking_id", event.BookingID, "user_id", event.UserID, "email", user.Email, "error", err)
		return
	}

	logger.InfoContext(ctx, "booking_confirmation_email_sent", "booking_id", event.BookingID, "user_id", event.UserID, "email", user.Email)
}

func buildBookingConfirmedEmailBody(event events.BookingEvent, booking *models.Booking, roomName string) string {
	start := "не указано"
	if event.SlotStart != nil {
		start = event.SlotStart.UTC().Format("2006-01-02 15:04 UTC")
	}
	end := "не указано"
	if event.SlotEnd != nil {
		end = event.SlotEnd.UTC().Format("2006-01-02 15:04 UTC")
	}

	body := "Бронирование переговорки подтверждено.\n\n" +
		"Переговорка: " + roomName + "\n" +
		"Начало: " + start + "\n" +
		"Окончание: " + end + "\n" +
		"ID брони: " + event.BookingID + "\n"

	if booking != nil && booking.ConferenceLink != nil && *booking.ConferenceLink != "" {
		body += "Ссылка конференции: " + *booking.ConferenceLink + "\n"
	}

	return body
}

func newRouter(appAPI *api.API, jwtSecret string, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(api.RequestLoggingMiddleware(logger))
	router.Use(api.RecoveryLoggingMiddleware(logger))

	router.Get("/_info", appAPI.HandleInfo)
	router.Handle("/metrics", promhttp.Handler())
	router.Post("/dummyLogin", appAPI.HandleDummyLogin)
	router.Post("/register", appAPI.HandleRegister)
	router.Post("/login", appAPI.HandleLogin)
	router.Post("/dbtest", appAPI.HandleDBTest)

	router.Group(func(router chi.Router) {
		router.Use(api.AuthMiddleware(jwtSecret))
		router.Get("/rooms/list", appAPI.HandleListRooms)
		router.Get("/rooms/{roomId}/slots/list", appAPI.HandleListSlots)
		router.With(api.RequireRoleMiddleware("admin")).Post("/rooms/create", appAPI.HandleCreateRoom)
		router.With(api.RequireRoleMiddleware("admin")).Post("/rooms/{roomId}/schedule/create", appAPI.HandleCreateSchedule)
		router.With(api.RequireRoleMiddleware("admin")).Get("/bookings/list", appAPI.HandleListAllBookings)
		router.With(api.RequireRoleMiddleware("user")).Post("/bookings/create", appAPI.HandleCreateBooking)
		router.With(api.RequireRoleMiddleware("user")).Get("/bookings/my", appAPI.HandleMyBookings)
		router.With(api.RequireRoleMiddleware("user")).Post("/bookings/{bookingId}/cancel", appAPI.HandleCancelBooking)
	})

	return router
}

func startSlotSync(ctx context.Context, scheduleUC usecases.ScheduleUseCase, logger *slog.Logger) {
	sync := func() {
		if err := scheduleUC.SyncAllSlots(ctx); err != nil {
			logger.ErrorContext(ctx, "slot_sync_failed", "error", err)
			return
		}
		logger.DebugContext(ctx, "slot_sync_completed")
	}

	sync()
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				sync()
			}
		}
	}()
}
