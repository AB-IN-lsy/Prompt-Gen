/*
 * @Author: NEFU AB-IN
 * @Date: 2025-10-10 17:00:39
 * @FilePath: \electron-go-app\backend\internal\infra\email\smtp_sender.go
 * @LastEditTime: 2025-10-10 17:00:43
 */
package email

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strings"
	"time"

	domain "electron-go-app/backend/internal/domain/user"
)

// Sender 实现 auth.EmailSender 接口。
type Sender struct {
	addr    string
	auth    smtp.Auth
	from    string
	baseURL string
	host    string
}

// NewSender 根据 Config 构造 SMTP 邮件发送器。
func NewSender(cfg SMTPConfig) (*Sender, error) {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return &Sender{
		addr:    addr,
		auth:    auth,
		from:    cfg.From,
		baseURL: strings.TrimRight(cfg.VerificationBaseURL, "/"),
		host:    cfg.Host,
	}, nil
}

// SendVerification 发送邮箱验证邮件。
func (s *Sender) SendVerification(ctx context.Context, user *domain.User, token string) error {
	if s == nil {
		return fmt.Errorf("smtp sender not configured")
	}

	subject, textBody, _ := composeVerificationContent(s.baseURL, user, token)

	msg := buildMessage(s.from, user.Email, subject, textBody)

	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.host)
	if err != nil {
		return fmt.Errorf("new smtp client: %w", err)
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		tlsConfig := &tls.Config{ServerName: s.host}
		if err := client.StartTLS(tlsConfig); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}

	if s.auth != nil {
		if ok, _ := client.Extension("AUTH"); ok {
			if err := client.Auth(s.auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}

	envelopeFrom := s.from
	if addr, err := mail.ParseAddress(s.from); err == nil && addr.Address != "" {
		envelopeFrom = addr.Address
	}

	if err := client.Mail(envelopeFrom); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(user.Email); err != nil {
		return fmt.Errorf("smtp rcpt to: %w", err)
	}

	writer, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}

	if _, err := writer.Write(msg); err != nil {
		_ = writer.Close()
		return fmt.Errorf("smtp write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("smtp close: %w", err)
	}

	return client.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	formattedFrom := formatAddress(from)
	formattedSubject := encodeHeader(subject)

	var buf bytes.Buffer
	buf.WriteString(fmt.Sprintf("From: %s\r\n", formattedFrom))
	buf.WriteString(fmt.Sprintf("To: %s\r\n", to))
	buf.WriteString(fmt.Sprintf("Subject: %s\r\n", formattedSubject))
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("\r\n")
	buf.WriteString(body)
	return buf.Bytes()
}

func formatAddress(raw string) string {
	addr, err := mail.ParseAddress(raw)
	if err != nil {
		return encodeHeader(raw)
	}
	if addr.Name != "" {
		addr.Name = encodeHeader(addr.Name)
	}
	return addr.String()
}

func encodeHeader(value string) string {
	if isASCII(value) {
		return value
	}
	return mime.QEncoding.Encode("UTF-8", value)
}

func isASCII(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] >= 0x80 {
			return false
		}
	}
	return true
}
