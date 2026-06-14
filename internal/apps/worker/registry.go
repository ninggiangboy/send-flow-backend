package worker

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"golang.org/x/sync/errgroup"
)

type Runner interface {
	Name() string
	Run(context.Context) error
}

type keyedRunner interface {
	RunnerKey() string
}

type Registry struct {
	runners map[string]Runner
}

func NewRegistry() *Registry {
	return &Registry{runners: map[string]Runner{}}
}

func (r *Registry) Register(runner Runner) error {
	name := strings.TrimSpace(runner.Name())
	if name == "" {
		return fmt.Errorf("worker runner name is required")
	}
	if _, exists := r.runners[name]; exists {
		return fmt.Errorf("worker runner %q is already registered", name)
	}
	r.runners[name] = runner
	return nil
}

func (r *Registry) Run(ctx context.Context, enabled []string, log *slog.Logger) error {
	selected, err := r.selectRunners(enabled)
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		log.Info("worker started with no enabled consumers")
		<-ctx.Done()
		return nil
	}

	group, groupCtx := errgroup.WithContext(ctx)
	for _, runner := range selected {
		runner := runner
		group.Go(func() error {
			log.Info("worker consumer starting", "consumer", runner.Name())
			return runner.Run(groupCtx)
		})
	}
	return group.Wait()
}

func (r *Registry) selectRunners(enabled []string) ([]Runner, error) {
	if len(enabled) == 0 {
		return nil, nil
	}
	selected := make([]Runner, 0, len(enabled))
	seen := map[string]struct{}{}
	for _, name := range enabled {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		runner, exists := r.runners[name]
		if !exists {
			return nil, fmt.Errorf("worker consumer %q is not registered", name)
		}
		key := runner.Name()
		if keyed, ok := runner.(keyedRunner); ok {
			key = keyed.RunnerKey()
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		selected = append(selected, runner)
	}
	return selected, nil
}
