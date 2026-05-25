package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

const (
	defaultBackendURL = "http://backend:8080"
	defaultInterval   = 2 * time.Second
	defaultPassword   = "loadgen-password"
)

type client struct {
	baseURL    string
	httpClient *http.Client
}

type tokenResponse struct {
	Token string `json:"token"`
}

type roomResponse struct {
	Room struct {
		ID string `json:"id"`
	} `json:"room"`
}

type slotsResponse struct {
	Slots []struct {
		ID    string    `json:"id"`
		Start time.Time `json:"start"`
	} `json:"slots"`
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	c := &client{
		baseURL: strings.TrimRight(getEnv("BACKEND_URL", defaultBackendURL), "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}

	interval := getDurationEnv("LOADGEN_INTERVAL", defaultInterval)
	adminEmail := getEnv("LOADGEN_ADMIN_EMAIL", "loadgen-admin@example.com")
	userEmail := getEnv("LOADGEN_USER_EMAIL", "loadgen-user@example.com")
	password := getEnv("LOADGEN_PASSWORD", defaultPassword)

	if err := waitForBackend(ctx, c, logger); err != nil {
		logger.Error("backend_wait_failed", "error", err)
		os.Exit(1)
	}

	adminToken, err := ensureUser(ctx, c, adminEmail, password, "admin")
	if err != nil {
		logger.Error("admin_auth_failed", "error", err)
		os.Exit(1)
	}
	userToken, err := ensureUser(ctx, c, userEmail, password, "user")
	if err != nil {
		logger.Error("user_auth_failed", "error", err)
		os.Exit(1)
	}

	roomID, err := prepareRoom(ctx, c, adminToken)
	if err != nil {
		logger.Error("load_room_prepare_failed", "error", err)
		os.Exit(1)
	}

	logger.Info("loadgen_started", "backend_url", c.baseURL, "interval", interval.String(), "room_id", roomID)

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Info("loadgen_stopped")
			return
		case <-ticker.C:
			if err := runScenario(ctx, c, userToken, roomID); err != nil {
				logger.Warn("loadgen_scenario_failed", "error", err)
			} else {
				logger.Info("loadgen_scenario_completed")
			}
		}
	}
}

func waitForBackend(ctx context.Context, c *client, logger *slog.Logger) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	deadline := time.NewTimer(2 * time.Minute)
	defer deadline.Stop()

	for {
		err := c.get(ctx, "/_info", "", nil)
		if err == nil {
			return nil
		}
		logger.Info("backend_not_ready", "error", err)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-deadline.C:
			return errors.New("backend readiness timeout")
		case <-ticker.C:
		}
	}
}

func ensureUser(ctx context.Context, c *client, email, password, role string) (string, error) {
	registerPayload := map[string]string{
		"email":    email,
		"password": password,
		"role":     role,
	}
	if err := c.post(ctx, "/register", "", registerPayload, nil); err != nil && !isStatus(err, http.StatusBadRequest) {
		return "", err
	}

	var token tokenResponse
	loginPayload := map[string]string{
		"email":    email,
		"password": password,
	}
	if err := c.post(ctx, "/login", "", loginPayload, &token); err != nil {
		return "", err
	}
	if token.Token == "" {
		return "", errors.New("empty login token")
	}
	return token.Token, nil
}

func prepareRoom(ctx context.Context, c *client, adminToken string) (string, error) {
	var room roomResponse
	roomPayload := map[string]interface{}{
		"name":        fmt.Sprintf("Load Room %d", time.Now().Unix()),
		"description": "Created by load generator",
		"capacity":    8,
	}
	if err := c.post(ctx, "/rooms/create", adminToken, roomPayload, &room); err != nil {
		return "", err
	}
	if room.Room.ID == "" {
		return "", errors.New("empty room id")
	}

	day := apiDay(time.Now().UTC().AddDate(0, 0, 1))
	schedulePayload := map[string]interface{}{
		"daysOfWeek": []int{day},
		"startTime":  "00:00",
		"endTime":    "23:30",
	}
	path := fmt.Sprintf("/rooms/%s/schedule/create", room.Room.ID)
	if err := c.post(ctx, path, adminToken, schedulePayload, nil); err != nil {
		return "", err
	}

	return room.Room.ID, nil
}

func runScenario(ctx context.Context, c *client, userToken, roomID string) error {
	if err := c.get(ctx, "/rooms/list", userToken, nil); err != nil {
		return err
	}

	date := time.Now().UTC().AddDate(0, 0, 1).Format("2006-01-02")
	var slots slotsResponse
	if err := c.get(ctx, fmt.Sprintf("/rooms/%s/slots/list?date=%s", roomID, date), userToken, &slots); err != nil {
		return err
	}
	if len(slots.Slots) == 0 {
		return errors.New("no available slots")
	}

	payload := map[string]interface{}{
		"slotId":               slots.Slots[0].ID,
		"createConferenceLink": true,
	}
	if err := c.post(ctx, "/bookings/create", userToken, payload, nil); err != nil && !isStatus(err, http.StatusConflict) {
		return err
	}

	return c.get(ctx, "/bookings/my", userToken, nil)
}

func (c *client) get(ctx context.Context, path, token string, target interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req, target)
}

func (c *client) post(ctx context.Context, path, token string, payload interface{}, target interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return c.do(req, target)
}

func (c *client) do(req *http.Request, target interface{}) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return statusError{status: resp.StatusCode, body: string(body)}
	}

	if target == nil || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, target)
}

type statusError struct {
	status int
	body   string
}

func (e statusError) Error() string {
	return fmt.Sprintf("unexpected status %d: %s", e.status, e.body)
}

func isStatus(err error, status int) bool {
	var statusErr statusError
	return errors.As(err, &statusErr) && statusErr.status == status
}

func apiDay(t time.Time) int {
	weekday := int(t.Weekday())
	if weekday == 0 {
		return 7
	}
	return weekday
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getDurationEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
