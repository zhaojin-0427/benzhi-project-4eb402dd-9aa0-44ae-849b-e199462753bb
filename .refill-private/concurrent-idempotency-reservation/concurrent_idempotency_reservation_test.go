package concurrentidempotencyreservation_test

import (
	"errors"
	"soundledger/internal/domain"
	"soundledger/internal/store"
	"testing"
	"time"
)

type marshalBarrier struct {
	entered chan<- struct{}
	release <-chan struct{}
}

func (b marshalBarrier) MarshalJSON() ([]byte, error) {
	b.entered <- struct{}{}
	<-b.release
	return []byte(`{"accepted":true}`), nil
}

func TestConcurrentIdempotencyKeyHasSingleCommit(t *testing.T) {
	repository, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 24, 8, 0, 0, 0, time.UTC)
	first := newBatch(t, "batch-first", now)
	second := newBatch(t, "batch-second", now)
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	results := make(chan error, 2)

	commit := func(aggregate *domain.Aggregate, requestID string) {
		results <- repository.Commit(store.CommitRequest{
			Aggregate:       aggregate,
			ExpectedVersion: 0,
			EventType:       "batch.created",
			ActorID:         "administrator",
			RequestID:       requestID,
			Payload:         map[string]any{"batchId": aggregate.Batch.ID},
			IdempotencyKey:  "shared-global-key",
			Operation:       "create_batch",
			Response:        marshalBarrier{entered: entered, release: release},
			Status:          201,
			OccurredAt:      now,
		})
	}

	go commit(first, "request-first")
	go commit(second, "request-second")
	<-entered
	<-entered
	close(release)

	successes := 0
	conflicts := 0
	for i := 0; i < 2; i++ {
		err = <-results
		switch {
		case err == nil:
			successes++
		case errors.Is(err, store.ErrIdempotencyConflict):
			conflicts++
		default:
			t.Fatalf("并发提交返回了非预期错误: %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("全局幂等键应只允许一次提交，得到成功 %d 次、冲突 %d 次、事件 %d 条", successes, conflicts, len(repository.Events("")))
	}
	if len(repository.Events("")) != 1 {
		t.Fatalf("全局幂等键冲突后事件日志应只有一条，得到 %d 条", len(repository.Events("")))
	}
}

func newBatch(t *testing.T, id string, now time.Time) *domain.Aggregate {
	t.Helper()
	aggregate, err := domain.NewBatch(
		id,
		"并发幂等预留测试",
		domain.SiteBoundary{North: 31.2, South: 31.1, East: 121.6, West: 121.5},
		domain.TimeWindow{Start: now, End: now.Add(time.Hour)},
		"科研授权",
		2,
		now,
	)
	if err != nil {
		t.Fatal(err)
	}
	return aggregate
}
