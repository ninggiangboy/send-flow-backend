package batching

import (
	"context"
	"runtime"
)

func Execute[I, O any](ctx context.Context, reader ItemReader[I], processor ItemProcessor[I, O], writer ItemWriter[O]) error {
	_, err := ExecuteSharded(ctx, 1, reader, processor, writer)
	return err
}

func ExecuteSharded[I, O any](ctx context.Context, totalShards int, reader ItemReader[I], processor ItemProcessor[I, O], writer ItemWriter[O]) ([]Stats, error) {
	cores := runtime.NumCPU()
	return ExecuteWithOptions(ctx, totalShards, reader, processor, writer, Options{
		BufferedItemsSize:    5000,
		WriteBatchSize:       500,
		ProcessorConcurrency: min(cores, 8),
		MaxInflight:          10000,
	})
}

func ExecuteCPUBound[I, O any](ctx context.Context, reader ItemReader[I], processor ItemProcessor[I, O], writer ItemWriter[O]) error {
	_, err := ExecuteCPUBoundSharded(ctx, 1, reader, processor, writer)
	return err
}

func ExecuteCPUBoundSharded[I, O any](ctx context.Context, totalShards int, reader ItemReader[I], processor ItemProcessor[I, O], writer ItemWriter[O]) ([]Stats, error) {
	cores := runtime.NumCPU()
	return ExecuteWithOptions(ctx, totalShards, reader, processor, writer, Options{
		BufferedItemsSize:    2000,
		WriteBatchSize:       200,
		ProcessorConcurrency: cores,
		MaxInflight:          4000,
	})
}

func ExecuteIOBound[I, O any](ctx context.Context, reader ItemReader[I], processor ItemProcessor[I, O], writer ItemWriter[O]) error {
	_, err := ExecuteIOBoundSharded(ctx, 1, reader, processor, writer)
	return err
}

func ExecuteIOBoundSharded[I, O any](ctx context.Context, totalShards int, reader ItemReader[I], processor ItemProcessor[I, O], writer ItemWriter[O]) ([]Stats, error) {
	cores := runtime.NumCPU()
	return ExecuteWithOptions(ctx, totalShards, reader, processor, writer, Options{
		BufferedItemsSize:    10000,
		WriteBatchSize:       100,
		ProcessorConcurrency: max(cores*4, 32),
		MaxInflight:          20000,
	})
}

func ExecuteWithOptions[I, O any](
	ctx context.Context,
	totalShards int,
	reader ItemReader[I],
	processor ItemProcessor[I, O],
	writer ItemWriter[O],
	opts Options,
) ([]Stats, error) {
	if totalShards <= 0 {
		totalShards = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	pipelines := make([]*Pipeline[I, O], 0, totalShards)
	for shardID := 0; shardID < totalShards; shardID++ {
		shardOpts := opts
		shardOpts.ShardID = shardID
		shardOpts.TotalShards = totalShards

		pipeline, err := NewPipeline(Config[I, O]{
			Options:   shardOpts,
			Reader:    reader,
			Processor: processor,
			Writer:    writer,
		})
		if err != nil {
			return nil, err
		}
		pipelines = append(pipelines, pipeline)
	}

	errCh := make(chan error, totalShards)
	for _, pipeline := range pipelines {
		p := pipeline
		go func() {
			if err := p.Run(ctx); err != nil {
				errCh <- err
				cancel()
				return
			}
			errCh <- nil
		}()
	}

	var firstErr error
	for range pipelines {
		if err := <-errCh; err != nil && firstErr == nil {
			firstErr = err
			cancel()
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}

	stats := make([]Stats, 0, len(pipelines))
	for _, pipeline := range pipelines {
		stats = append(stats, pipeline.Stats())
	}
	return stats, nil
}
