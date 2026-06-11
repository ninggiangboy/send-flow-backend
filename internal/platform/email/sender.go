package email

import (
	"context"
	"fmt"
)

type Message struct {
	To      []string
	Subject string
	Text    string
	HTML    string
}

func (m Message) Validate() error {
	if len(m.To) == 0 {
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
