package batching

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
)

type ItemReader[I any] interface {
	Read(ctx context.Context, shardID, totalShards int) (item I, ok bool, err error)
}

type ItemReaderFunc[I any] func(ctx context.Context, shardID, totalShards int) (I, bool, error)

func (f ItemReaderFunc[I]) Read(ctx context.Context, shardID, totalShards int) (I, bool, error) {
	return f(ctx, shardID, totalShards)
}

type ItemProcessor[I, O any] interface {
	Process(ctx context.Context, item I) (out O, ok bool, err error)
}

type ItemProcessorFunc[I, O any] func(ctx context.Context, item I) (O, bool, error)

func (f ItemProcessorFunc[I, O]) Process(ctx context.Context, item I) (O, bool, error) {
	return f(ctx, item)
}

type ItemWriter[O any] interface {
	Write(ctx context.Context, batch []O) error
}

type ItemWriterFunc[O any] func(ctx context.Context, batch []O) error

func (f ItemWriterFunc[O]) Write(ctx context.Context, batch []O) error {
	return f(ctx, batch)
}

type Options struct {
	ShardID              int
	TotalShards          int
	BufferedItemsSize    int
	WriteBatchSize       int
	ProcessorConcurrency int
	MaxInflight          int
}

type Stats struct {
	ItemsRead      int64
	ItemsProcessed int64
	ItemsWritten   int64
}

type Pipeline[I, O any] struct {
	opts      Options
	reader    ItemReader[I]
	processor ItemProcessor[I, O]
	writer    ItemWriter[O]

	itemsRead      atomic.Int64
	itemsProcessed atomic.Int64
	itemsWritten   atomic.Int64
}

type Config[I, O any] struct {
	Options
	Reader    ItemReader[I]
	Processor ItemProcessor[I, O]
	Writer    ItemWriter[O]
}

func NewPipeline[I, O any](cfg Config[I, O]) (*Pipeline[I, O], error) {
	opts := withDefaults(cfg.Options)
	if err := validateOptions(opts); err != nil {
		return nil, err
	}
	if cfg.Reader == nil {
		return nil, errors.New("batching: reader is required")
	}
	if cfg.Processor == nil {
		return nil, errors.New("batching: processor is required")
	}
	if cfg.Writer == nil {
		return nil, errors.New("batching: writer is required")
	}

	return &Pipeline[I, O]{
		opts:      opts,
		reader:    cfg.Reader,
		processor: cfg.Processor,
		writer:    cfg.Writer,
	}, nil
}

func (p *Pipeline[I, O]) Run(ctx context.Context) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	inputCh := make(chan I, p.opts.BufferedItemsSize)
	outputCh := make(chan O, p.opts.BufferedItemsSize)
	inflight := make(chan struct{}, p.opts.MaxInflight)
	errCh := make(chan error, p.opts.ProcessorConcurrency+2)

	var processorWG sync.WaitGroup
	var allWG sync.WaitGroup

	fail := func(err error) {
		if err == nil {
			return
		}
		select {
		case errCh <- err:
			cancel()
		default:
			cancel()
		}
	}

	allWG.Add(1)
	go func() {
		defer allWG.Done()
		defer close(inputCh)
		if err := p.runReader(ctx, inputCh, inflight); err != nil {
			fail(err)
		}
	}()

	processorWG.Add(p.opts.ProcessorConcurrency)
	for i := 0; i < p.opts.ProcessorConcurrency; i++ {
		allWG.Add(1)
		go func() {
			defer allWG.Done()
			defer processorWG.Done()
			if err := p.runProcessor(ctx, inputCh, outputCh, inflight); err != nil {
				fail(err)
			}
		}()
	}

	allWG.Add(1)
	go func() {
		defer allWG.Done()
		processorWG.Wait()
		close(outputCh)
	}()

	allWG.Add(1)
	go func() {
		defer allWG.Done()
		if err := p.runWriter(ctx, outputCh, inflight); err != nil {
			fail(err)
		}
	}()

	done := make(chan struct{})
	go func() {
		allWG.Wait()
		close(done)
	}()

	select {
	case <-done:
		select {
		case err := <-errCh:
			return err
		default:
			return ctx.Err()
		}
	case err := <-errCh:
		cancel()
		<-done
		return err
	}
}

func (p *Pipeline[I, O]) Stats() Stats {
	return Stats{
		ItemsRead:      p.itemsRead.Load(),
		ItemsProcessed: p.itemsProcessed.Load(),
		ItemsWritten:   p.itemsWritten.Load(),
	}
}

func (p *Pipeline[I, O]) runReader(ctx context.Context, inputCh chan<- I, inflight chan struct{}) error {
	for {
		item, ok, err := p.reader.Read(ctx, p.opts.ShardID, p.opts.TotalShards)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}

		select {
		case inflight <- struct{}{}:
		case <-ctx.Done():
			return ctx.Err()
		}

		select {
		case inputCh <- item:
			p.itemsRead.Add(1)
		case <-ctx.Done():
			releaseInflight(inflight, 1)
			return ctx.Err()
		}
	}
}

func (p *Pipeline[I, O]) runProcessor(ctx context.Context, inputCh <-chan I, outputCh chan<- O, inflight chan struct{}) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-inputCh:
			if !ok {
				return nil
			}

			out, keep, err := p.processor.Process(ctx, item)
			if err != nil {
				releaseInflight(inflight, 1)
				return err
			}
			if !keep {
				releaseInflight(inflight, 1)
				continue
			}

			select {
			case outputCh <- out:
				p.itemsProcessed.Add(1)
			case <-ctx.Done():
				releaseInflight(inflight, 1)
				return ctx.Err()
			}
		}
	}
}

func (p *Pipeline[I, O]) runWriter(ctx context.Context, outputCh <-chan O, inflight chan struct{}) error {
	buffer := make([]O, 0, p.opts.WriteBatchSize)
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case item, ok := <-outputCh:
			if !ok {
				if len(buffer) == 0 {
					return nil
				}
				return p.flush(ctx, buffer, inflight)
			}

			buffer = append(buffer, item)
			if len(buffer) >= p.opts.WriteBatchSize {
				if err := p.flush(ctx, buffer, inflight); err != nil {
					return err
				}
				buffer = buffer[:0]
			}
		}
	}
}

func (p *Pipeline[I, O]) flush(ctx context.Context, buffer []O, inflight chan struct{}) error {
	batch := append([]O(nil), buffer...)
	if err := p.writer.Write(ctx, batch); err != nil {
		releaseInflight(inflight, len(batch))
		return err
	}
	p.itemsWritten.Add(int64(len(batch)))
	releaseInflight(inflight, len(batch))
	return nil
}

func releaseInflight(inflight chan struct{}, n int) {
	for i := 0; i < n; i++ {
		select {
		case <-inflight:
		default:
			return
		}
	}
}

func withDefaults(opts Options) Options {
	if opts.TotalShards == 0 {
		opts.TotalShards = 1
	}
	if opts.BufferedItemsSize == 0 {
		opts.BufferedItemsSize = 5000
	}
	if opts.WriteBatchSize == 0 {
		opts.WriteBatchSize = 500
	}
	if opts.ProcessorConcurrency == 0 {
		opts.ProcessorConcurrency = min(runtime.NumCPU(), 8)
	}
	if opts.MaxInflight == 0 {
		opts.MaxInflight = 10000
	}
	return opts
}

func validateOptions(opts Options) error {
	if opts.TotalShards <= 0 {
		return errors.New("batching: total shards must be > 0")
	}
	if opts.ShardID < 0 || opts.ShardID >= opts.TotalShards {
		return fmt.Errorf("batching: shard id must be in [0,%d)", opts.TotalShards)
	}
	if opts.BufferedItemsSize <= 0 {
		return errors.New("batching: buffered items size must be > 0")
	}
	if opts.WriteBatchSize <= 0 {
		return errors.New("batching: write batch size must be > 0")
	}
	if opts.ProcessorConcurrency <= 0 {
		return errors.New("batching: processor concurrency must be > 0")
	}
	if opts.MaxInflight < opts.WriteBatchSize {
		return errors.New("batching: max inflight must be >= write batch size")
	}
	return nil
}
