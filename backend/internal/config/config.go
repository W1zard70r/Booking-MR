package config

import (
	"fmt"
	"os"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	JWT      JWTConfig
	Logging  LoggingConfig
	RabbitMQ RabbitMQConfig
	SMTP     SMTPConfig
}

type ServerConfig struct {
	Port string
}

type DatabaseConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
	SSLMode  string
}

type JWTConfig struct {
	Secret string
}

type LoggingConfig struct {
	Level  string
	Format string
}

type RabbitMQConfig struct {
	URL                      string
	BookingEventsQueue       string
	BookingStatusEventsQueue string
}

type SMTPConfig struct {
	Enabled  bool
	Host     string
	Port     string
	Username string
	Password string
	From     string
	UseTLS   bool
}

func Load() Config {
	return Config{
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "password"),
			Name:     getEnv("DB_NAME", "booking"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		JWT: JWTConfig{
			Secret: getEnv("JWT_SECRET", "change-me"),
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
		RabbitMQ: RabbitMQConfig{
			URL:                      getEnv("RABBITMQ_URL", ""),
			BookingEventsQueue:       getEnv("RABBITMQ_BOOKING_EVENTS_QUEUE", "booking-events"),
			BookingStatusEventsQueue: getEnv("RABBITMQ_BOOKING_STATUS_EVENTS_QUEUE", "booking-status-events"),
		},
		SMTP: SMTPConfig{
			Enabled:  getEnvBool("SMTP_ENABLED", false),
			Host:     getEnv("SMTP_HOST", ""),
			Port:     getEnv("SMTP_PORT", "587"),
			Username: getEnv("SMTP_USERNAME", ""),
			Password: getEnv("SMTP_PASSWORD", ""),
			From:     getEnv("SMTP_FROM", ""),
			UseTLS:   getEnvBool("SMTP_USE_TLS", true),
		},
	}
}

func (c DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		c.Host,
		c.Port,
		c.User,
		c.Password,
		c.Name,
		c.SSLMode,
	)
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func getEnvBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value == "true" || value == "1" || value == "yes"
}
