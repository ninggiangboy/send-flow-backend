package email

import (
	"bytes"
	"context"
	"encoding/base64"
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
	if err := msg.Validate(); err != nil {
		return err
	}

	mime, err := buildMIMEMessage(s.from, msg)
	if err != nil {
		return err
	}

	// SMTP envelope recipients: all To + CC + BCC.
	// BCC recipients appear in the envelope (RCPT TO) but NOT in the MIME headers,
	// preserving recipient privacy.
	allRecipients := make([]string, 0, len(msg.To)+len(msg.CC)+len(msg.BCC))
	allRecipients = append(allRecipients, msg.To...)
	allRecipients = append(allRecipients, msg.CC...)
	allRecipients = append(allRecipients, msg.BCC...)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	errCh := make(chan error, 1)
	go func() {
		errCh <- smtp.SendMail(addr, s.auth, s.from, allRecipients, mime)
	}()

	select {
	case <-ctx.Done():
		go func() {
			<-errCh
		}()
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

	// --- Headers ---
	if msg.SenderName != "" {
		b.WriteString(fmt.Sprintf("From: %s <%s>\r\n", msg.SenderName, from))
	} else {
		b.WriteString(fmt.Sprintf("From: %s\r\n", from))
	}
	if len(msg.To) > 0 {
		b.WriteString(fmt.Sprintf("To: %s\r\n", strings.Join(msg.To, ",")))
	} else {
		// RFC 5322 allows omitted To when recipients are in CC/BCC only.
		// Use an undisclosed-recipients placeholder to satisfy strict parsers.
		b.WriteString("To: undisclosed-recipients:;\r\n")
	}
	if len(msg.CC) > 0 {
		b.WriteString(fmt.Sprintf("Cc: %s\r\n", strings.Join(msg.CC, ",")))
	}
	// BCC is intentionally excluded from MIME headers for privacy.
	b.WriteString(fmt.Sprintf("Subject: %s\r\n", msg.Subject))
	b.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().UTC().Format(time.RFC1123Z)))
	b.WriteString("MIME-Version: 1.0\r\n")

	// Custom headers (e.g. Reply-To, X-*)
	for k, v := range msg.Headers {
		if strings.HasPrefix(k, "Content-") || strings.HasPrefix(k, "MIME-") {
			continue // skip transport-level headers
		}
		b.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}

	hasAttachments := len(msg.Attachments) > 0
	hasAlternative := msg.Text != "" && msg.HTML != ""

	if hasAttachments {
		mixedBoundary := fmt.Sprintf("sendflow-mixed-%d", time.Now().UnixNano())
		b.WriteString(fmt.Sprintf("Content-Type: multipart/mixed; boundary=%q\r\n\r\n", mixedBoundary))

		// Inner body part — multipart/alternative if both text and HTML, else single.
		if hasAlternative {
			altBoundary := fmt.Sprintf("sendflow-alt-%d", time.Now().UnixNano())
			b.WriteString("--" + mixedBoundary + "\r\n")
			b.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=%q\r\n\r\n", altBoundary))

			b.WriteString("--" + altBoundary + "\r\n")
			b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
			b.WriteString(msg.Text + "\r\n")

			b.WriteString("--" + altBoundary + "\r\n")
			b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
			b.WriteString(msg.HTML + "\r\n")

			b.WriteString("--" + altBoundary + "--\r\n")
		} else {
			b.WriteString("--" + mixedBoundary + "\r\n")
			if msg.HTML != "" {
				b.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
				b.WriteString(msg.HTML + "\r\n")
			} else {
				b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n\r\n")
				b.WriteString(msg.Text + "\r\n")
			}
		}

		// Attachments
		for _, att := range msg.Attachments {
			ct := att.ContentType
			if ct == "" {
				ct = "application/octet-stream"
			}
			disp := att.Disposition
			if disp == "" {
				disp = "attachment"
			}

			b.WriteString("--" + mixedBoundary + "\r\n")
			b.WriteString(fmt.Sprintf("Content-Type: %s\r\n", ct))
			b.WriteString(fmt.Sprintf("Content-Disposition: %s; filename=%q\r\n", disp, att.Filename))
			if att.ContentID != "" {
				b.WriteString(fmt.Sprintf("Content-ID: <%s>\r\n", att.ContentID))
			}
			b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")

			encoded := base64.StdEncoding.EncodeToString(att.Data)
			for i := 0; i < len(encoded); i += 76 {
				end := i + 76
				if end > len(encoded) {
					end = len(encoded)
				}
				b.WriteString(encoded[i:end] + "\r\n")
			}
		}

		b.WriteString("--" + mixedBoundary + "--\r\n")
		return b.Bytes(), nil
	}

	if hasAlternative {
		boundary := fmt.Sprintf("sendflow-%d", time.Now().UnixNano())
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
