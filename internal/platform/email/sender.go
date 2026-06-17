package email

import (
	"context"
	"fmt"
)

// Attachment represents a file attached to an email message.
type Attachment struct {
	Filename    string
	ContentType string
	ContentID   string
	Disposition string
	Data        []byte
}

type Message struct {
	To          []string
	CC          []string
	BCC         []string
	Subject     string
	Text        string
	HTML        string
	SenderName  string
	Headers     map[string]string
	Attachments []Attachment
}

func (m Message) Validate() error {
	if len(m.To) == 0 && len(m.CC) == 0 && len(m.BCC) == 0 {
		return fmt.Errorf("email recipient is required")
	}
	if m.Subject == "" {
		return fmt.Errorf("email subject is required")
	}
	if m.Text == "" && m.HTML == "" {
		return fmt.Errorf("email body is required")
	}
	return nil
}

type Sender interface {
	Send(ctx context.Context, msg Message) error
}
