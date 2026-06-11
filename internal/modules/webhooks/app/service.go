package app

import (
	"log/slog"
	"time"

	"github.com/ninggiangboy/send-flow/backend/internal/modules/webhooks/ports"
)

type Options struct {
	ConfigRead    ports.ConfigReadRepository
	ConfigWrite   ports.ConfigWriteRepository
	DeliveryRead  ports.DeliveryReadRepository
	DeliveryWrite ports.DeliveryWriteRepository
	AttemptWrite  ports.AttemptWriteRepository
	AttemptRead   ports.AttemptReadRepository
	TxManager     ports.TransactionManager
	OutboxWriter  ports.OutboxWriter
	Deliverer     ports.HTTPDeliverer
	AccessChecker ports.WorkspaceAccessChecker
	IDGen         func() (string, error)
	Clock         func() time.Time
	Logger        *slog.Logger
}

type Service struct {
	configRead    ports.ConfigReadRepository
	configWrite   ports.ConfigWriteRepository
	deliveryRead  ports.DeliveryReadRepository
	deliveryWrite ports.DeliveryWriteRepository
	attemptWrite  ports.AttemptWriteRepository
	attemptRead   ports.AttemptReadRepository
	txManager     ports.TransactionManager
	outboxWriter  ports.OutboxWriter
	deliverer     ports.HTTPDeliverer
	accessChecker ports.WorkspaceAccessChecker
	idGen         func() (string, error)
	clock         func() time.Time
	log           *slog.Logger
}

func NewService(opts Options) *Service {
	if opts.Clock == nil {
		opts.Clock = time.Now
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	return &Service{
		configRead:    opts.ConfigRead,
		configWrite:   opts.ConfigWrite,
		deliveryRead:  opts.DeliveryRead,
		deliveryWrite: opts.DeliveryWrite,
		attemptWrite:  opts.AttemptWrite,
		attemptRead:   opts.AttemptRead,
		txManager:     opts.TxManager,
		outboxWriter:  opts.OutboxWriter,
		deliverer:     opts.Deliverer,
		accessChecker: opts.AccessChecker,
		idGen:         opts.IDGen,
		clock:         opts.Clock,
		log:           opts.Logger.With("module", "webhooks"),
	}
}
