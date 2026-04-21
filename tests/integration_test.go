package tests

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"room-booking/internal/api"
	"room-booking/internal/repository"
	"room-booking/internal/usecases"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// setupTestServer инициализирует все слои приложения для тестов.
func setupTestServer(t *testing.T) (http.Handler, *api.API, string) {
	_ = godotenv.Load("../.env")

	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		getEnv("DB_HOST", "localhost"), getEnv("DB_PORT", "5437"),
		getEnv("DB_USER", "postgres"), getEnv("DB_PASSWORD", "password"),
		getEnv("DB_NAME", "booking"), getEnv("DB_SSLMODE", "disable"),
	)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to DB: %v", err)
	}

	repo := repository.New(db)
	jwtSecret := "test-secret-key"

	roomUC := usecases.NewRoomUseCase(repo.Room)
	schedUC := usecases.NewScheduleUseCase(repo.Schedule, repo.Slot)
	slotUC := usecases.NewSlotUseCase(repo.Slot, repo.Room)
	bookUC := usecases.NewBookingUseCase(repo.Booking, repo.Slot)
	userUC := usecases.NewUserUseCase(repo.User, jwtSecret)

	appAPI := &api.API{
		RoomUC:     roomUC,
		ScheduleUC: schedUC,
		SlotUC:     slotUC,
		BookingUC:  bookUC,
		UserUC:     userUC,
		JWTSecret:  jwtSecret,
	}

	r := chi.NewRouter()

	r.Get("/_info", appAPI.HandleInfo)
	r.Post("/dummyLogin", appAPI.HandleDummyLogin)
	r.Post("/register", appAPI.HandleRegister)
	r.Post("/login", appAPI.HandleLogin)

	r.Group(func(r chi.Router) {
		r.Use(api.AuthMiddleware(jwtSecret))
		r.Get("/rooms/list", appAPI.HandleListRooms)
		r.With(api.RequireRoleMiddleware("admin")).Post("/rooms/create", appAPI.HandleCreateRoom)
		r.With(api.RequireRoleMiddleware("admin")).Post("/rooms/{roomId}/schedule/create", appAPI.HandleCreateSchedule)
		r.Get("/rooms/{roomId}/slots/list", appAPI.HandleListSlots)
		r.With(api.RequireRoleMiddleware("user")).Post("/bookings/create", appAPI.HandleCreateBooking)
		r.With(api.RequireRoleMiddleware("user")).Get("/bookings/my", appAPI.HandleMyBookings)
		r.With(api.RequireRoleMiddleware("user")).Post("/bookings/{bookingId}/cancel", appAPI.HandleCancelBooking)
		r.With(api.RequireRoleMiddleware("admin")).Get("/bookings/list", appAPI.HandleListAllBookings)
	})

	return r, appAPI, jwtSecret
}

// --- 1. АВТОРИЗАЦИЯ ---

func TestAuthWorkflow_E2E(t *testing.T) {
	r, _, _ := setupTestServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	email := fmt.Sprintf("test-%d@example.com", time.Now().UnixNano())
	password := "password123"

	regBody, _ := json.Marshal(map[string]string{"email": email, "password": password, "role": "user"})
	resp, _ := http.Post(ts.URL+"/register", "application/json", bytes.NewBuffer(regBody))
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("Registration failed: %d", resp.StatusCode)
	}

	loginBody, _ := json.Marshal(map[string]string{"email": email, "password": password})
	resp, _ = http.Post(ts.URL+"/login", "application/json", bytes.NewBuffer(loginBody))
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Login failed: %d", resp.StatusCode)
	}
}

// --- 2. ПОЛНЫЙ ЦИКЛ БРОНИРОВАНИЯ ---

func TestFullBookingWorkflow_E2E(t *testing.T) {
	r, _, _ := setupTestServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	adminToken := getToken(t, ts, "admin")
	userToken := getToken(t, ts, "user")

	roomID := createRoom(t, ts, adminToken, "Workflow Room")

	// Расписание на Четверг (4)
	schedBody, _ := json.Marshal(map[string]interface{}{
		"daysOfWeek": []int{4}, "startTime": "10:00", "endTime": "11:00",
	})
	req, _ := http.NewRequest("POST", ts.URL+"/rooms/"+roomID+"/schedule/create", bytes.NewBuffer(schedBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	http.DefaultClient.Do(req)

	date := getNextDateForWeekday(4)
	req, _ = http.NewRequest("GET", ts.URL+"/rooms/"+roomID+"/slots/list?date="+date, nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ := http.DefaultClient.Do(req)
	var slotsRes struct{ Slots []struct{ ID string } }
	json.NewDecoder(resp.Body).Decode(&slotsRes)
	slotID := slotsRes.Slots[0].ID

	bookBody, _ := json.Marshal(map[string]string{"slotId": slotID})
	req, _ = http.NewRequest("POST", ts.URL+"/bookings/create", bytes.NewBuffer(bookBody))
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ = http.DefaultClient.Do(req)

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Booking failed, got %d", resp.StatusCode)
	}
}

// --- 3. ОТМЕНА И ИДЕМПОТЕНТНОСТЬ ---

func TestCancelBooking_E2E(t *testing.T) {
	r, _, _ := setupTestServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	adminToken := getToken(t, ts, "admin")
	userToken := getToken(t, ts, "user")

	roomID := createRoom(t, ts, adminToken, "Cancel Room")
	// Пятница (5)
	schedBody, _ := json.Marshal(map[string]interface{}{"daysOfWeek": []int{5}, "startTime": "10:00", "endTime": "11:00"})
	req, _ := http.NewRequest("POST", ts.URL+"/rooms/"+roomID+"/schedule/create", bytes.NewBuffer(schedBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	http.DefaultClient.Do(req)

	date := getNextDateForWeekday(5)
	req, _ = http.NewRequest("GET", ts.URL+"/rooms/"+roomID+"/slots/list?date="+date, nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ := http.DefaultClient.Do(req)
	var slotsRes struct{ Slots []struct{ ID string } }
	json.NewDecoder(resp.Body).Decode(&slotsRes)
	slotID := slotsRes.Slots[0].ID

	bookBody, _ := json.Marshal(map[string]string{"slotId": slotID})
	req, _ = http.NewRequest("POST", ts.URL+"/bookings/create", bytes.NewBuffer(bookBody))
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ = http.DefaultClient.Do(req)
	var bookRes struct{ Booking struct{ ID string } }
	json.NewDecoder(resp.Body).Decode(&bookRes)
	bookingID := bookRes.Booking.ID

	// Тест: Отмена 1
	req, _ = http.NewRequest("POST", ts.URL+"/bookings/"+bookingID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Errorf("Cancel 1 failed: %d", resp.StatusCode)
	}

	// Тест: Отмена 2 (Идемпотентность)
	req, _ = http.NewRequest("POST", ts.URL+"/bookings/"+bookingID+"/cancel", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Errorf("Cancel 2 failed: %d", resp.StatusCode)
	}
}

// --- 4. СПИСКИ, ПАГИНАЦИЯ И ДОСТУП (ВОССТАНОВЛЕНО) ---

func TestListsAndPermissions_E2E(t *testing.T) {
	r, _, _ := setupTestServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	adminToken := getToken(t, ts, "admin")
	userToken := getToken(t, ts, "user")

	// Создаем комнату для теста списка
	createRoom(t, ts, adminToken, "List Test Room")

	// Тест: Список комнат (доступен всем)
	req, _ := http.NewRequest("GET", ts.URL+"/rooms/list", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ := http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Error("Failed to list rooms")
	}

	// Тест: Список всех броней (Admin only, пагинация)
	req, _ = http.NewRequest("GET", ts.URL+"/bookings/list?page=1&pageSize=10", nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Error("Admin failed to list all bookings")
	}

	// Тест: Ошибка доступа (User лезет в список всех броней)
	req, _ = http.NewRequest("GET", ts.URL+"/bookings/list", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 403 {
		t.Errorf("Expected 403 for user, got %d", resp.StatusCode)
	}

	// Тест: Мои брони (User)
	req, _ = http.NewRequest("GET", ts.URL+"/bookings/my", nil)
	req.Header.Set("Authorization", "Bearer "+userToken)
	resp, _ = http.DefaultClient.Do(req)
	if resp.StatusCode != 200 {
		t.Error("Failed to get my bookings")
	}

	// Тест: Info (Public)
	resp, _ = http.Get(ts.URL + "/_info")
	if resp.StatusCode != 200 {
		t.Error("Info endpoint failed")
	}
}

// --- 5. ФОНОВЫЙ ВОРКЕР ---

func TestSlotWorkerSync_Integration(t *testing.T) {
	r, apiObj, _ := setupTestServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	adminToken := getToken(t, ts, "admin")
	roomID := createRoom(t, ts, adminToken, "Worker Room")

	// Суббота (6)
	schedBody, _ := json.Marshal(map[string]interface{}{"daysOfWeek": []int{6}, "startTime": "08:00", "endTime": "09:00"})
	req, _ := http.NewRequest("POST", ts.URL+"/rooms/"+roomID+"/schedule/create", bytes.NewBuffer(schedBody))
	req.Header.Set("Authorization", "Bearer "+adminToken)
	http.DefaultClient.Do(req)

	// Получаем текущее кол-во слотов
	date := getNextDateForWeekday(6)
	req, _ = http.NewRequest("GET", ts.URL+"/rooms/"+roomID+"/slots/list?date="+date, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, _ := http.DefaultClient.Do(req)
	var res struct{ Slots []interface{} }
	json.NewDecoder(resp.Body).Decode(&res)
	countBefore := len(res.Slots)

	// Запуск воркера вручную
	if err := apiObj.ScheduleUC.SyncAllSlots(context.Background()); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	// Проверка: кол-во не выросло (нет дублей)
	req, _ = http.NewRequest("GET", ts.URL+"/rooms/"+roomID+"/slots/list?date="+date, nil)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	resp, _ = http.DefaultClient.Do(req)
	json.NewDecoder(resp.Body).Decode(&res)
	if len(res.Slots) != countBefore {
		t.Errorf("Duplicates created! Before: %d, After: %d", countBefore, len(res.Slots))
	}
}

// --- ХЕЛПЕРЫ ---

func getToken(t *testing.T, ts *httptest.Server, role string) string {
	body, _ := json.Marshal(map[string]string{"role": role})
	resp, _ := http.Post(ts.URL+"/dummyLogin", "application/json", bytes.NewBuffer(body))
	var res struct{ Token string }
	json.NewDecoder(resp.Body).Decode(&res)
	return res.Token
}

func createRoom(t *testing.T, ts *httptest.Server, token, name string) string {
	body, _ := json.Marshal(map[string]string{"name": name})
	req, _ := http.NewRequest("POST", ts.URL+"/rooms/create", bytes.NewBuffer(body))
	req.Header.Set("Authorization", "Bearer "+token)
	resp, _ := http.DefaultClient.Do(req)
	var res struct{ Room struct{ ID string } }
	json.NewDecoder(resp.Body).Decode(&res)
	return res.Room.ID
}

func getNextDateForWeekday(targetWeekday int) string {
	now := time.Now().UTC()
	target := time.Weekday(targetWeekday % 7)
	for i := 0; i < 7; i++ {
		date := now.AddDate(0, 0, i)
		if date.Weekday() == target {
			return date.Format("2006-01-02")
		}
	}
	return now.Format("2006-01-02")
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}

// --- 6. ТЕСТЫ ОШИБОК ---

func TestErrorScenarios_E2E(t *testing.T) {
	r, _, _ := setupTestServer(t)
	ts := httptest.NewServer(r)
	defer ts.Close()

	adminToken := getToken(t, ts, "admin")
	userToken := getToken(t, ts, "user")

	// 1. Создаем "другого" пользователя (для теста Forbidden)
	otherEmail := fmt.Sprintf("other-%d@ex.com", time.Now().UnixNano())
	regBody, _ := json.Marshal(map[string]string{"email": otherEmail, "password": "123", "role": "user"})
	http.Post(ts.URL+"/register", "application/json", bytes.NewBuffer(regBody))

	loginBody, _ := json.Marshal(map[string]string{"email": otherEmail, "password": "123"})
	respL, err := http.Post(ts.URL+"/login", "application/json", bytes.NewBuffer(loginBody))
	if err != nil {
		t.Fatalf("Login failed: %v", err)
	}

	var resL struct{ Token string }
	json.NewDecoder(respL.Body).Decode(&resL)
	otherUserToken := resL.Token

	// --- Тест: Unauthorized ---
	t.Run("Unauthorized_NoToken", func(t *testing.T) {
		req, _ := http.NewRequest("GET", ts.URL+"/rooms/list", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("Request failed: %v", err)
		}
		if resp.StatusCode != 401 {
			t.Errorf("Expected 401, got %d", resp.StatusCode)
		}
	})

	// Подготовка данных
	roomID := createRoom(t, ts, adminToken, "Error Room")
	schedBody, _ := json.Marshal(map[string]interface{}{"daysOfWeek": []int{3}, "startTime": "12:00", "endTime": "14:00"})
	reqS, _ := http.NewRequest("POST", ts.URL+"/rooms/"+roomID+"/schedule/create", bytes.NewBuffer(schedBody))
	reqS.Header.Set("Authorization", "Bearer "+adminToken)
	http.DefaultClient.Do(reqS)

	date := getNextDateForWeekday(3)
	reqSlots, _ := http.NewRequest("GET", ts.URL+"/rooms/"+roomID+"/slots/list?date="+date, nil)
	reqSlots.Header.Set("Authorization", "Bearer "+userToken)
	respSlots, _ := http.DefaultClient.Do(reqSlots)

	var slotsRes struct{ Slots []struct{ ID string } }
	json.NewDecoder(respSlots.Body).Decode(&slotsRes)

	if len(slotsRes.Slots) < 2 {
		t.Fatalf("Need at least 2 slots, got %d", len(slotsRes.Slots))
	}
	slotID := slotsRes.Slots[0].ID

	// --- Тест: Конфликт бронирования ---
	t.Run("Booking_Conflict", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"slotId": slotID})

		// ПЕРВАЯ БРОНЬ
		req1, _ := http.NewRequest("POST", ts.URL+"/bookings/create", bytes.NewBuffer(body))
		req1.Header.Set("Authorization", "Bearer "+userToken)
		http.DefaultClient.Do(req1)

		// ВТОРАЯ БРОНЬ (Создаем НОВЫЙ объект запроса, нельзя переиспользовать req1)
		req2, _ := http.NewRequest("POST", ts.URL+"/bookings/create", bytes.NewBuffer(body))
		req2.Header.Set("Authorization", "Bearer "+userToken)
		resp2, err := http.DefaultClient.Do(req2)

		if err != nil {
			t.Fatalf("Second booking attempt failed: %v", err)
		}
		if resp2.StatusCode != 409 {
			t.Errorf("Expected 409 Conflict, got %d", resp2.StatusCode)
		}
	})

	// --- Тест: Слот не найден ---
	t.Run("Booking_NotFound", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"slotId": "00000000-0000-0000-0000-000000000000"})
		req, _ := http.NewRequest("POST", ts.URL+"/bookings/create", bytes.NewBuffer(body))
		req.Header.Set("Authorization", "Bearer "+userToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp.StatusCode != 404 {
			t.Errorf("Expected 404, got %d", resp.StatusCode)
		}
	})

	// --- Тест: Чужая бронь (Forbidden) ---
	t.Run("Cancel_Forbidden", func(t *testing.T) {
		anotherSlotID := slotsRes.Slots[1].ID
		body, _ := json.Marshal(map[string]string{"slotId": anotherSlotID})

		reqB, _ := http.NewRequest("POST", ts.URL+"/bookings/create", bytes.NewBuffer(body))
		reqB.Header.Set("Authorization", "Bearer "+userToken)
		respB, _ := http.DefaultClient.Do(reqB)

		var bRes struct{ Booking struct{ ID string } }
		json.NewDecoder(respB.Body).Decode(&bRes)

		// Пытаемся отменить чужим токеном
		reqC, _ := http.NewRequest("POST", ts.URL+"/bookings/"+bRes.Booking.ID+"/cancel", nil)
		reqC.Header.Set("Authorization", "Bearer "+otherUserToken)
		respC, err := http.DefaultClient.Do(reqC)

		if err != nil {
			t.Fatalf("Cancel request failed: %v", err)
		}
		if respC.StatusCode != 403 {
			t.Errorf("Expected 403 Forbidden, got %d", respC.StatusCode)
		}
	})

	// --- Тест: Повторное расписание ---
	t.Run("Schedule_Conflict", func(t *testing.T) {
		req, _ := http.NewRequest("POST", ts.URL+"/rooms/"+roomID+"/schedule/create", bytes.NewBuffer(schedBody))
		req.Header.Set("Authorization", "Bearer "+adminToken)
		resp, _ := http.DefaultClient.Do(req)
		if resp.StatusCode != 409 {
			t.Errorf("Expected 409, got %d", resp.StatusCode)
		}
	})
}
