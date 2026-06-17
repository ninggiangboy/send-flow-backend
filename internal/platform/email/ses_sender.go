package email

import (
	"context"
	"fmt"
	"strings"

	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"
	"github.com/ninggiangboy/send-flow/backend/internal/platform/config"
)

type SESSender struct {
	client           *sesv2.Client
	from             string
	configurationSet string
}

func NewSESSender(ctx context.Context, cfg config.SESConfig) (*SESSender, error) {
	loadOptions := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" && cfg.SecretKey != "" {
		loadOptions = append(loadOptions, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretKey, cfg.SessionToken)))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, loadOptions...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	if cfg.Endpoint != "" {
		return &SESSender{
			client: sesv2.NewFromConfig(awsCfg, func(o *sesv2.Options) { o.BaseEndpoint = &cfg.Endpoint }),
			from:   cfg.From, configurationSet: cfg.Configuration,
		}, nil
	}

	return &SESSender{client: sesv2.NewFromConfig(awsCfg), from: cfg.From, configurationSet: cfg.Configuration}, nil
}

func (s *SESSender) Send(ctx context.Context, msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}

	// Check if we need raw MIME — needed when there are attachments or
	// custom headers that don't fit in the Simple format (anything beyond Reply-To).
	needsRaw := len(msg.Attachments) > 0
	if !needsRaw {
		for k := range msg.Headers {
			if !strings.EqualFold(k, "Reply-To") && !strings.HasPrefix(k, "Content-") && !strings.HasPrefix(k, "MIME-") {
				needsRaw = true
				break
			}
		}
	}

	if needsRaw {
		return s.sendRaw(ctx, msg)
	}
	return s.sendSimple(ctx, msg)
}

func (s *SESSender) sendSimple(ctx context.Context, msg Message) error {
	fromAddr := s.from
	if msg.SenderName != "" {
		fromAddr = fmt.Sprintf("%s <%s>", msg.SenderName, s.from)
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: &fromAddr,
		Destination: &types.Destination{
			ToAddresses:  msg.To,
			CcAddresses:  msg.CC,
			BccAddresses: msg.BCC,
		},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: &msg.Subject},
			Body:    &types.Body{},
		}},
	}

	// Reply-To from headers.
	if replyTo, ok := msg.Headers["Reply-To"]; ok {
		parts := strings.Split(replyTo, ",")
		addrs := make([]string, 0, len(parts))
		for _, p := range parts {
			if addr := strings.TrimSpace(p); addr != "" {
				addrs = append(addrs, addr)
			}
		}
		if len(addrs) > 0 {
			input.ReplyToAddresses = addrs
		}
	}

	if msg.Text != "" {
		input.Content.Simple.Body.Text = &types.Content{Data: &msg.Text}
	}
	if msg.HTML != "" {
		input.Content.Simple.Body.Html = &types.Content{Data: &msg.HTML}
	}
	if s.configurationSet != "" {
		input.ConfigurationSetName = &s.configurationSet
	}

	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return fmt.Errorf("send ses email: %w", err)
	}
	return nil
}

func (s *SESSender) sendRaw(ctx context.Context, msg Message) error {
	mimeBytes, err := buildMIMEMessage(s.from, msg)
	if err != nil {
		return fmt.Errorf("build mime message: %w", err)
	}

	input := &sesv2.SendEmailInput{
		FromEmailAddress: &s.from,
		Destination: &types.Destination{
			ToAddresses:  msg.To,
			CcAddresses:  msg.CC,
			BccAddresses: msg.BCC,
		},
		Content: &types.EmailContent{
			Raw: &types.RawMessage{Data: mimeBytes},
		},
	}

	// Also pass Reply-To at the SES API level so it's available to SES features (bounces, etc.).
	if replyTo, ok := msg.Headers["Reply-To"]; ok {
		parts := strings.Split(replyTo, ",")
		addrs := make([]string, 0, len(parts))
		for _, p := range parts {
			if addr := strings.TrimSpace(p); addr != "" {
				addrs = append(addrs, addr)
			}
		}
		if len(addrs) > 0 {
			input.ReplyToAddresses = addrs
		}
	}

	if s.configurationSet != "" {
		input.ConfigurationSetName = &s.configurationSet
	}

	if _, err := s.client.SendEmail(ctx, input); err != nil {
		return fmt.Errorf("send ses email: %w", err)
	}
	return nil
}
