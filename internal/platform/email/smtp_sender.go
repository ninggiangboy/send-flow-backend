package email

import (
	"bytes"
	"context"
	"fmt"
	"net/smtp"
	"strings"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

type SMTPSender struct {
	host string
	port int
	from string
	auth smtp.Auth
}

func NewSMTPSender(cfg config.SMTPConfig) *SMTPSender {
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return &SMTPSender{host: cfg.Host, port: cfg.Port, from: cfg.From, auth: auth}
}

func (s *SMTPSender) Send(ctx context.Context, msg Message) error {
	if len(msg.To) == 0 {
		return fmt.Errorf("email recipient is required")
	}
	if msg.Subject == "" {
		return fmt.Errorf("email subject is required")
	}
	if msg.Text == "" && msg.HTML == "" {
		return fmt.Errorf("email body is required")
	}

	mime, err := buildMIMEMessage(s.from, msg)
	if err != nil {
		return err
	}

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, s.auth, s.from, msg.To, mime)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("send smtp email: %w", err)
		}
		return nil
	}
}

func buildMIMEMessage(from string, msg Message) ([]byte, error) {
	var b bytes.Buffer
	boundary := fmt.Sprintf("sendflow-%d", time.Now().UnixNano())

	b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ",")))
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	b.WriteString("MIME-Version: 1.0\r\n")

	if msg.Text != "" && msg.HTML != "" {
		b.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n\r\n", boundary))
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.Text + "\r\n")
		b.WriteString("--" + boundary + "\r\n")
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.HTML + "\r\n")
		b.WriteString("--" + boundary + "--\r\n")
		return b.Bytes(), nil
	}

	if msg.HTML != "" {
		b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
		b.WriteString(msg.HTML)
		return b.Bytes(), nil
	}

	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
	b.WriteString(msg.Text)
	return b.Bytes(), nil
}
