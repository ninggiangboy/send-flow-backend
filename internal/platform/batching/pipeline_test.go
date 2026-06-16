package batching

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
)

func TestPipelineProcessesAndWritesBatches(t *testing.T) {
	var next atomic.Int64
	var mu sync.Mutex
	var got []int
	var batchSizes []int

	reader := ItemReaderFunc[int](func(ctx context.Context, shardID, totalShards int) (int, bool, error) {
		v := int(next.Add(1))
		if v > 10 {
			return 0, false, nil
		}
		return v, true, nil
	})
	processor := ItemProcessorFunc[int, int](func(ctx context.Context, item int) (int, bool, error) {
		return item * 2, true, nil
	})
	writer := ItemWriterFunc[int](func(ctx context.Context, batch []int) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, batch...)
		batchSizes = append(batchSizes, len(batch))
		return nil
	})

	pipeline, err := NewPipeline(Config[int, int]{
		Options: Options{
			WriteBatchSize:       3,
			BufferedItemsSize:    4,
			ProcessorConcurrency: 2,
			MaxInflight:          6,
		},
		Reader:    reader,
		Processor: processor,
		Writer:    writer,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := pipeline.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	slices.Sort(got)
	want := []int{2, 4, 6, 8, 10, 12, 14, 16, 18, 20}
	if !slices.Equal(got, want) {
		t.Fatalf("unexpected written items: got %v want %v", got, want)
	}

	for _, size := range batchSizes {
		if size > 3 {
			t.Fatalf("batch too large: %d", size)
		}
	}

	stats := pipeline.Stats()
	if stats.ItemsRead != 10 || stats.ItemsProcessed != 10 || stats.ItemsWritten != 10 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestPipelineDropsProcessorOutput(t *testing.T) {
	var next atomic.Int64
	var mu sync.Mutex
	var got []int

	reader := ItemReaderFunc[int](func(ctx context.Context, shardID, totalShards int) (int, bool, error) {
		v := int(next.Add(1))
		if v > 6 {
			return 0, false, nil
		}
		return v, true, nil
	})
	processor := ItemProcessorFunc[int, int](func(ctx context.Context, item int) (int, bool, error) {
		if item%2 == 0 {
			return 0, false, nil
		}
		return item, true, nil
	})
	writer := ItemWriterFunc[int](func(ctx context.Context, batch []int) error {
		mu.Lock()
		defer mu.Unlock()
		got = append(got, batch...)
		return nil
	})

	pipeline, err := NewPipeline(Config[int, int]{
		Options: Options{
			WriteBatchSize:       2,
			BufferedItemsSize:    2,
			ProcessorConcurrency: 2,
			MaxInflight:          4,
		},
		Reader:    reader,
		Processor: processor,
		Writer:    writer,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := pipeline.Run(context.Background()); err != nil {
		t.Fatal(err)
	}

	slices.Sort(got)
	if !slices.Equal(got, []int{1, 3, 5}) {
		t.Fatalf("unexpected written items: %v", got)
	}

	stats := pipeline.Stats()
	if stats.ItemsRead != 6 || stats.ItemsProcessed != 3 || stats.ItemsWritten != 3 {
		t.Fatalf("unexpected stats: %+v", stats)
	}
}

func TestPipelineReturnsProcessorError(t *testing.T) {
	wantErr := errors.New("processor failed")
	var next atomic.Int64

	reader := ItemReaderFunc[int](func(ctx context.Context, shardID, totalShards int) (int, bool, error) {
		v := int(next.Add(1))
		if v > 5 {
			return 0, false, nil
		}
		return v, true, nil
	})
	processor := ItemProcessorFunc[int, int](func(ctx context.Context, item int) (int, bool, error) {
		if item == 3 {
			return 0, false, wantErr
		}
		return item, true, nil
	})
	writer := ItemWriterFunc[int](func(ctx context.Context, batch []int) error {
		return nil
	})

	pipeline, err := NewPipeline(Config[int, int]{
		Options: Options{
			WriteBatchSize:       2,
			BufferedItemsSize:    2,
			ProcessorConcurrency: 1,
			MaxInflight:          4,
		},
		Reader:    reader,
		Processor: processor,
		Writer:    writer,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := pipeline.Run(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("expected processor error, got %v", err)
	}
}

func TestExecuteWithOptionsRunsShards(t *testing.T) {
	const totalShards = 3
	limits := []int{4, 3, 5}
	counts := make([]int, totalShards)
	var countsMu sync.Mutex
	seen := make(map[int]bool)
	var seenMu sync.Mutex

	reader := ItemReaderFunc[int](func(ctx context.Context, shardID, totalShards int) (int, bool, error) {
		countsMu.Lock()
		defer countsMu.Unlock()

		if counts[shardID] >= limits[shardID] {
			return 0, false, nil
		}
		counts[shardID]++
		return shardID*100 + counts[shardID], true, nil
	})
	processor := ItemProcessorFunc[int, int](func(ctx context.Context, item int) (int, bool, error) {
		return item, true, nil
	})
	writer := ItemWriterFunc[int](func(ctx context.Context, batch []int) error {
		seenMu.Lock()
		defer seenMu.Unlock()
		for _, item := range batch {
			seen[item] = true
		}
		return nil
	})

	stats, err := ExecuteWithOptions(context.Background(), totalShards, reader, processor, writer, Options{
		WriteBatchSize:       2,
		BufferedItemsSize:    3,
		ProcessorConcurrency: 2,
		MaxInflight:          5,
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(stats) != totalShards {
		t.Fatalf("expected %d stats entries, got %d", totalShards, len(stats))
	}

	totalWritten := int64(0)
	for _, stat := range stats {
		totalWritten += stat.ItemsWritten
	}
	if totalWritten != 12 {
		t.Fatalf("expected 12 written items, got %d", totalWritten)
	}

	for shardID, limit := range limits {
		for i := 1; i <= limit; i++ {
			item := shardID*100 + i
			if !seen[item] {
				t.Fatalf("missing item %d", item)
			}
		}
	}
}

func TestNewPipelineValidatesOptions(t *testing.T) {
	reader := ItemReaderFunc[int](func(ctx context.Context, shardID, totalShards int) (int, bool, error) {
		return 0, false, nil
	})
	processor := ItemProcessorFunc[int, int](func(ctx context.Context, item int) (int, bool, error) {
		return item, true, nil
	})
	writer := ItemWriterFunc[int](func(ctx context.Context, batch []int) error {
		return nil
	})

	_, err := NewPipeline(Config[int, int]{
		Options: Options{
			WriteBatchSize:       10,
			BufferedItemsSize:    1,
			ProcessorConcurrency: 1,
			MaxInflight:          5,
		},
		Reader:    reader,
		Processor: processor,
		Writer:    writer,
	})
	if err == nil {
		t.Fatal("expected validation error")
	}
}
