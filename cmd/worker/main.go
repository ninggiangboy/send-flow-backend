package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/ninggiangboy/send-flow/backend/internal/apps/worker"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if err := worker.Run(ctx); err != nil {
		log.Fatal(err)
	}
}
