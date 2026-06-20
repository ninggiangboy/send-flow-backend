package app

import (
	"context"
	"log/slog"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/app/provider"
	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/ports"
)

type Options struct {
	RawEventsRead         ports.RawEventReadRepository
	RawEventsWrite        ports.RawEventWriteRepository
	NormalizedEventsRead  ports.NormalizedEventReadRepository
	NormalizedEventsWrite ports.NormalizedEventWriteRepository
	ProviderRegistry      ports.ProviderRegistry
	MessageResolver       ports.DeliveryMessageResolver
	OutboxWriter          ports.OutboxWriter
	TxManager             ports.TransactionManager
	IDGen                 func() (string, error)
	Logger                *slog.Logger
}

type Service struct {
	ingestProviderWebhookH *provider.Handler
	log                    *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		ingestProviderWebhookH: provider.New(provider.Options{
			RawEventsRead:         opts.RawEventsRead,
			RawEventsWrite:        opts.RawEventsWrite,
			NormalizedEventsRead:  opts.NormalizedEventsRead,
			NormalizedEventsWrite: opts.NormalizedEventsWrite,
			ProviderRegistry:      opts.ProviderRegistry,
			MessageResolver:       opts.MessageResolver,
			OutboxWriter:          opts.OutboxWriter,
			TxManager:             opts.TxManager,
			IDGen:                 opts.IDGen,
			Logger:                opts.Logger,
		}),
		log: opts.Logger.With("module", "ingestion"),
	}
}

// IngestProviderWebhookInput is defined in types.go via type alias to provider.Input.
// IngestProviderWebhookResult is defined in types.go via type alias to provider.Result.

func (s *Service) IngestProviderWebhook(ctx context.Context, input IngestProviderWebhookInput) (*IngestProviderWebhookResult, error) {
	return s.ingestProviderWebhookH.Execute(ctx, input)
}
