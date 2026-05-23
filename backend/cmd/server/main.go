package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"time"

	"room-booking/internal/api"
	"room-booking/internal/config"
	"room-booking/internal/logging"
	"room-booking/internal/repository"
	"room-booking/internal/usecases"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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

	repo := repository.New(db)
	roomUC := usecases.NewRoomUseCase(repo.Room, logger)
	scheduleUC := usecases.NewScheduleUseCase(repo.Schedule, repo.Slot, logger)
	slotUC := usecases.NewSlotUseCase(repo.Slot, repo.Room)
	bookingUC := usecases.NewBookingUseCase(repo.Booking, repo.Slot, logger)
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
	startSlotSync(context.Background(), scheduleUC, logger)

	server := &http.Server{
		Addr:              ":" + cfg.Server.Port,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	logger.Info(
		"server started",
		"port", cfg.Server.Port,
		"log_level", cfg.Logging.Level,
		"log_format", cfg.Logging.Format,
	)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("server stopped unexpectedly", "error", err)
		os.Exit(1)
	}
}

func newRouter(appAPI *api.API, jwtSecret string, logger *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(api.RequestLoggingMiddleware(logger))
	router.Use(api.RecoveryLoggingMiddleware(logger))

	router.Get("/_info", appAPI.HandleInfo)
	router.Post("/dummyLogin", appAPI.HandleDummyLogin)
	router.Post("/register", appAPI.HandleRegister)
	router.Post("/login", appAPI.HandleLogin)

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
			logger.ErrorContext(ctx, "slot sync failed", "error", err)
			return
		}
		logger.DebugContext(ctx, "slot sync completed")
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
