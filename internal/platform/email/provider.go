package email

import (
	"context"
	"fmt"
	"strings"

	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

func NewSender(ctx context.Context, cfg config.Config) (Sender, error) {
	switch strings.ToLower(cfg.EmailProvider) {
	case "smtp":
		return NewSMTPSender(cfg.SMTP), nil
	case "ses":
		return NewSESSender(ctx, cfg.SES)
	default:
		return nil, fmt.Errorf("unsupported email provider: %s", cfg.EmailProvider)
	}
}
