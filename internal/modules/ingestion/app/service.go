package app

import (
	"context"
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/ingestion/app/ingestproviderwebhook"
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
	ingestProviderWebhookH *ingestproviderwebhook.Handler
	log                    *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		ingestProviderWebhookH: ingestproviderwebhook.New(ingestproviderwebhook.Options{
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

type IngestProviderWebhookInput struct {
	Provider   string
	Headers    map[string][]string
	RawBody    []byte
	ReceivedAt time.Time
}

type IngestProviderWebhookResult struct {
	Accepted          bool
	RawEventID        string
	NormalizedEventID string
}

func (s *Service) IngestProviderWebhook(ctx context.Context, input IngestProviderWebhookInput) (*IngestProviderWebhookResult, error) {
	result, err := s.ingestProviderWebhookH.Execute(ctx, ingestproviderwebhook.Input{
		Provider:   input.Provider,
		Headers:    input.Headers,
		RawBody:    input.RawBody,
		ReceivedAt: input.ReceivedAt,
	})
	if err != nil {
		return nil, err
	}
	return &IngestProviderWebhookResult{
		Accepted:          result.Accepted,
		RawEventID:        result.RawEventID,
		NormalizedEventID: result.NormalizedEventID,
	}, nil
}
