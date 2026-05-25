package notifications

import (
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"strings"
	"time"

	"room-booking/internal/config"
)

type EmailMessage struct {
	To      string
	Subject string
	Body    string
}

type EmailSender interface {
	Send(ctx context.Context, message EmailMessage) error
}

type NoopEmailSender struct{}

func (NoopEmailSender) Send(context.Context, EmailMessage) error {
	return nil
}

type SMTPEmailSender struct {
	host     string
	port     string
	username string
	password string
	from     string
	useTLS   bool
}

func NewEmailSender(cfg config.SMTPConfig) EmailSender {
	if !cfg.Enabled || cfg.Host == "" || cfg.Port == "" || cfg.From == "" {
		return NoopEmailSender{}
	}
	return &SMTPEmailSender{
		host:     cfg.Host,
		port:     cfg.Port,
		username: cfg.Username,
		password: cfg.Password,
		from:     cfg.From,
		useTLS:   cfg.UseTLS,
	}
}

func (s *SMTPEmailSender) Send(ctx context.Context, message EmailMessage) error {
	if message.To == "" {
		return fmt.Errorf("email recipient is required")
	}

	addr := net.JoinHostPort(s.host, s.port)
	headers := map[string]string{
		"From":         s.from,
		"To":           message.To,
		"Subject":      mime.QEncoding.Encode("utf-8", message.Subject),
		"MIME-Version": "1.0",
		"Content-Type": "text/plain; charset=UTF-8",
	}

	var builder strings.Builder
	for key, value := range headers {
		builder.WriteString(key)
		builder.WriteString(": ")
		builder.WriteString(value)
		builder.WriteString("\r\n")
	}
	builder.WriteString("\r\n")
	builder.WriteString(message.Body)

	conn, err := s.dial(ctx, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		if err := conn.SetDeadline(deadline); err != nil {
			return err
		}
	} else {
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	}

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return err
	}
	defer client.Close()

	if s.useTLS && s.port != "465" {
		tlsConfig := &tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		}
		if ok, _ := client.Extension("STARTTLS"); ok {
			if err := client.StartTLS(tlsConfig); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("smtp server does not support STARTTLS")
		}
	}

	if s.username != "" || s.password != "" {
		if err := client.Auth(plainAuth{username: s.username, password: s.password}); err != nil {
			return err
		}
	}

	if err := client.Mail(s.from); err != nil {
		return err
	}
	if err := client.Rcpt(message.To); err != nil {
		return err
	}

	writer, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := writer.Write([]byte(builder.String())); err != nil {
		_ = writer.Close()
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	return client.Quit()
}

func (s *SMTPEmailSender) dial(ctx context.Context, addr string) (net.Conn, error) {
	dialer := net.Dialer{}
	if s.useTLS && s.port == "465" {
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			return nil, err
		}
		tlsConn := tls.Client(conn, &tls.Config{
			ServerName: s.host,
			MinVersion: tls.VersionTLS12,
		})
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return tlsConn, nil
	}
	return dialer.DialContext(ctx, "tcp", addr)
}

type plainAuth struct {
	username string
	password string
}

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	response := "\x00" + a.username + "\x00" + a.password
	return "PLAIN", []byte(response), nil
}

func (a plainAuth) Next([]byte, bool) ([]byte, error) {
	return nil, nil
}
