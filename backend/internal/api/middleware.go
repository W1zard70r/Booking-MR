package api

import (
	"context"
	"net/http"
	"strings"

	"room-booking/pkg/auth"
)

type contextKey string

const (
	UserIDKey contextKey = "user_id"
	RoleKey   contextKey = "role"
)

// AuthMiddleware проверяет наличие JWT токена
func AuthMiddleware(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// получаем из заголовка токен
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "missing authorization header"}}`, http.StatusUnauthorized)
				return
			}

			// извлечение токена
			parts := strings.Split(authHeader, " ")
			if len(parts) != 2 || parts[0] != "Bearer" {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "invalid authorization format"}}`, http.StatusUnauthorized)
				return
			}

			claims, err := auth.ParseToken(parts[1], secret)
			if err != nil {
				http.Error(w, `{"error": {"code": "UNAUTHORIZED", "message": "invalid token"}}`, http.StatusUnauthorized)
				return
			}

			// Кладем userID и role в context запроса
			ctx := context.WithValue(r.Context(), UserIDKey, claims.UserID)
			ctx = context.WithValue(ctx, RoleKey, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRoleMiddleware проверяет роль пользователя
func RequireRoleMiddleware(requiredRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(RoleKey).(string)
			if !ok || role != requiredRole {
				http.Error(w, `{"error": {"code": "FORBIDDEN", "message": "access denied"}}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
