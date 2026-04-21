package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"room-booking/internal/api"
	"room-booking/internal/repository"
	"room-booking/internal/usecases"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

func main() {
	_ = godotenv.Load()

	// Подключение к БД
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"), getEnv("DB_PORT", "5437"),
		getEnv("DB_USER", "postgres"), getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "booking"), getEnv("DB_SSLMODE", "disable"),
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// Инициализация слоев
	repo := repository.New(db)

	roomUC := usecases.NewRoomUseCase(repo.Room)
	schedUC := usecases.NewScheduleUseCase(repo.Schedule, repo.Slot)
	slotUC := usecases.NewSlotUseCase(repo.Slot, repo.Room)
	bookUC := usecases.NewBookingUseCase(repo.Booking, repo.Slot)

	// Запуск фонового воркера для скользящего окна слотов
	go startSlotWorker(schedUC)

	jwtSecret := getEnv("JWT_SECRET", "supersecret")

	userUC := usecases.NewUserUseCase(repo.User, jwtSecret)
	handler := &api.API{
		RoomUC:     roomUC,
		ScheduleUC: schedUC,
		SlotUC:     slotUC,
		BookingUC:  bookUC,
		JWTSecret:  jwtSecret,
		UserUC:     userUC,
	}

	// Роутер
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Публичные эндпоинты
	r.Get("/_info", handler.HandleInfo)
	r.Post("/dummyLogin", handler.HandleDummyLogin)
	r.Post("/register", handler.HandleRegister)
	r.Post("/login", handler.HandleLogin)

	// Защищенные эндпоинты
	r.Group(func(r chi.Router) {
		r.Use(api.AuthMiddleware(jwtSecret))

		// Rooms (admin create, all list)
		r.Get("/rooms/list", handler.HandleListRooms)
		r.With(api.RequireRoleMiddleware("admin")).Post("/rooms/create", handler.HandleCreateRoom)

		// Schedules (admin only)
		r.With(api.RequireRoleMiddleware("admin")).Post("/rooms/{roomId}/schedule/create", handler.HandleCreateSchedule)

		// Slots (all)
		r.Get("/rooms/{roomId}/slots/list", handler.HandleListSlots)

		// Bookings
		r.With(api.RequireRoleMiddleware("user")).Post("/bookings/create", handler.HandleCreateBooking)
		r.With(api.RequireRoleMiddleware("user")).Get("/bookings/my", handler.HandleMyBookings)
		r.With(api.RequireRoleMiddleware("user")).Post("/bookings/{bookingId}/cancel", handler.HandleCancelBooking)
		r.With(api.RequireRoleMiddleware("admin")).Get("/bookings/list", handler.HandleListAllBookings)
	})

	port := getEnv("SERVER_PORT", "8080")
	fmt.Printf("🚀 Server starting on :%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func getEnv(key, fallback string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return fallback
}

func startSlotWorker(uc usecases.ScheduleUseCase) {
	// Запускаем сразу при старте, чтобы дозаполнить окно если сервер лежал
	log.Println("Syncing slots on startup...")
	if err := uc.SyncAllSlots(context.Background()); err != nil {
		log.Printf("Worker error: %v", err)
	}

	// Далее запускаем по тикеру (раз в 1 час — оптимально для RPS 100)
	ticker := time.NewTicker(1 * time.Hour)
	for range ticker.C {
		log.Println("Background slot sync started...")
		err := uc.SyncAllSlots(context.Background())
		if err != nil {
			log.Printf("Worker error during sync: %v", err)
		}
	}
}
