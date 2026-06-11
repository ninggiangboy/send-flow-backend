package email

import (
	"context"
	"fmt"

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

	input := &sesv2.SendEmailInput{
		FromEmailAddress: &s.from,
		Destination:      &types.Destination{ToAddresses: msg.To},
		Content: &types.EmailContent{Simple: &types.Message{
			Subject: &types.Content{Data: &msg.Subject},
			Body:    &types.Body{},
		}},
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
