package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"room-booking/internal/models"
	"room-booking/internal/repository"
	"room-booking/internal/usecases"
	"room-booking/pkg/auth"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
)

const (
	AdminStaticUUID = "11111111-1111-1111-1111-111111111111"
	UserStaticUUID  = "22222222-2222-2222-2222-222222222222"
)

type API struct {
	RoomUC     usecases.RoomUseCase
	ScheduleUC usecases.ScheduleUseCase
	SlotUC     usecases.SlotUseCase
	BookingUC  usecases.BookingUseCase
	JWTSecret  string
	UserUC     usecases.UserUseCase
	TestUC     usecases.TestUseCase
}

// --- СЛУЖЕБНЫЕ ---

func (api *API) HandleInfo(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"status": "ok"}`))
}

type DummyLoginReq struct {
	Role string `json:"role"`
}
type TokenResp struct {
	Token string `json:"token"`
}

func (api *API) HandleDummyLogin(w http.ResponseWriter, r *http.Request) {
	var req DummyLoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}

	var userID string
	if req.Role == "admin" {
		userID = AdminStaticUUID
	} else if req.Role == "user" {
		userID = UserStaticUUID
	} else {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "role must be admin or user")
		return
	}

	token, err := auth.GenerateToken(userID, req.Role, api.JWTSecret)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to generate token")
		return
	}

	respondJSON(w, http.StatusOK, TokenResp{Token: token})
}

// --- ROOMS ---

func (api *API) HandleCreateRoom(w http.ResponseWriter, r *http.Request) {
	var room models.Room
	if err := json.NewDecoder(r.Body).Decode(&room); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}

	if err := api.RoomUC.CreateRoom(r.Context(), &room); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"room": room})
}

func (api *API) HandleListRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := api.RoomUC.GetRooms(r.Context())
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"rooms": rooms})
}

// --- SCHEDULES ---

type CreateScheduleReq struct {
	DaysOfWeek []int  `json:"daysOfWeek"`
	StartTime  string `json:"startTime"`
	EndTime    string `json:"endTime"`
}

func (api *API) HandleCreateSchedule(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")

	var sched models.Schedule
	if err := json.NewDecoder(r.Body).Decode(&sched); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}
	sched.RoomID = roomID

	err := api.ScheduleUC.CreateSchedule(r.Context(), &sched)
	if err != nil {
		switch {
		case errors.Is(err, usecases.ErrInvalidDayOfWeek), errors.Is(err, usecases.ErrInvalidTimeRange):
			respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		case errors.Is(err, repository.ErrAlreadyExists):
			respondError(w, http.StatusConflict, "SCHEDULE_EXISTS", "schedule for this room already exists")
		default:
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}

	respondJSON(w, http.StatusCreated, map[string]interface{}{"schedule": sched})
}

// --- SLOTS ---

func (api *API) HandleListSlots(w http.ResponseWriter, r *http.Request) {
	roomID := chi.URLParam(r, "roomId")
	dateStr := r.URL.Query().Get("date")
	if dateStr == "" {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "date parameter is required")
		return
	}

	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid date format (use YYYY-MM-DD)")
		return
	}

	slots, err := api.SlotUC.GetAvailableSlots(r.Context(), roomID, date)
	if err != nil {
		if errors.Is(err, usecases.ErrRoomNotFound) {
			respondError(w, http.StatusNotFound, "ROOM_NOT_FOUND", err.Error())
			return
		}
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"slots": slots})
}

// --- BOOKINGS ---

type CreateBookingReq struct {
	SlotID               string `json:"slotId"`
	CreateConferenceLink bool   `json:"createConferenceLink"`
}

func (api *API) HandleCreateBooking(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	var req CreateBookingReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}

	booking, err := api.BookingUC.CreateBooking(r.Context(), userID, req.SlotID, req.CreateConferenceLink)
	if err != nil {
		if errors.Is(err, usecases.ErrSlotNotFound) {
			respondError(w, http.StatusNotFound, "SLOT_NOT_FOUND", err.Error())
		} else if errors.Is(err, usecases.ErrSlotInPast) {
			respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		} else if errors.Is(err, repository.ErrSlotAlreadyBooked) {
			respondError(w, http.StatusConflict, "SLOT_ALREADY_BOOKED", err.Error())
		} else {
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"booking": booking})
}

func (api *API) HandleCancelBooking(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	bookingID := chi.URLParam(r, "bookingId")

	booking, err := api.BookingUC.CancelBooking(r.Context(), userID, bookingID)
	if err != nil {
		if errors.Is(err, usecases.ErrBookingNotFound) {
			respondError(w, http.StatusNotFound, "BOOKING_NOT_FOUND", err.Error())
		} else if errors.Is(err, usecases.ErrForbidden) {
			respondError(w, http.StatusForbidden, "FORBIDDEN", "cannot cancel another user's booking")
		} else {
			respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		}
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"booking": booking})
}

func (api *API) HandleListAllBookings(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	pageSizeStr := r.URL.Query().Get("pageSize")

	page, _ := strconv.Atoi(pageStr)
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(pageSizeStr)
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	bookings, total, err := api.BookingUC.GetAllBookings(r.Context(), page, pageSize)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	respondJSON(w, http.StatusOK, map[string]interface{}{
		"bookings": bookings,
		"pagination": map[string]int{
			"page":     page,
			"pageSize": pageSize,
			"total":    total,
		},
	})
}

func (api *API) HandleMyBookings(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(UserIDKey).(string)
	bookings, err := api.BookingUC.GetMyBookings(r.Context(), userID)
	if err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]interface{}{"bookings": bookings})
}

// --- Users ---

type RegisterReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (api *API) HandleRegister(w http.ResponseWriter, r *http.Request) {
	var req RegisterReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}

	user, err := api.UserUC.Register(r.Context(), req.Email, req.Password, req.Role)
	if err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", err.Error())
		return
	}
	respondJSON(w, http.StatusCreated, map[string]interface{}{"user": user})
}

type LoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (api *API) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req LoginReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}

	token, err := api.UserUC.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		respondError(w, http.StatusUnauthorized, "UNAUTHORIZED", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, TokenResp{Token: token})
}

// -- Test --
type DBTestReq struct {
	Message string `json:"message"`
}

func (api *API) HandleDBTest(w http.ResponseWriter, r *http.Request) {
	var req DBTestReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "INVALID_REQUEST", "invalid body")
		return
	}

	if err := api.TestUC.Save(r.Context(), req.Message); err != nil {
		respondError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}
