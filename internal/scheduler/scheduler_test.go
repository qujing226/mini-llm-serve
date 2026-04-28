package scheduler

import (
	"testing"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
)

func newTestScheduler() *scheduler {
	return &scheduler{
		maxBatchSeqs:           8,
		maxBatchTokens:         16,
		maxPartialPrefills:     2,
		maxLongPartialPrefills: 1,
		longPrefillThreshold:   8,
		prefillQueueSmall:      NewPrefillQueue(testQueueConf(16)),
		prefillQueueLarge:      NewPrefillQueue(testQueueConf(16)),
		decodeQueue:            NewDecodeQueue(testQueueConf(16)),
	}
}

func requirePickBatchReturns(t *testing.T, s *scheduler) ([]*model.WorkItem, int) {
	t.Helper()

	type result struct {
		items []*model.WorkItem
		n     int
	}
	ch := make(chan result, 1)
	go func() {
		items, n := s.pickBatch()
		ch <- result{items: items, n: n}
	}()

	select {
	case got := <-ch:
		return got.items, got.n
	case <-time.After(200 * time.Millisecond):
		t.Fatal("pickBatch did not return")
		return nil, 0
	}
}

func TestPickBatchSchedulesDecodeBeforePrefill(t *testing.T) {
	s := newTestScheduler()

	require.NoError(t, s.decodeQueue.Enqueue(&model.WorkItem{
		WorkId: "d1",
		Phase:  v1.WorkPhaseDecode,
	}))
	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  4,
		PrefillTokens: 4,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "p1", items[1].WorkId)
	require.Equal(t, uint64(0), s.decodeQueue.Length())
	require.Equal(t, uint64(0), s.prefillQueueSmall.Length())
}

func TestPickBatchUsesRemainingTokenBudgetForSmallPrefill(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 6
	s.maxBatchSeqs = 4

	require.NoError(t, s.decodeQueue.Enqueue(&model.WorkItem{
		WorkId: "d1",
		Phase:  v1.WorkPhaseDecode,
	}))
	require.NoError(t, s.decodeQueue.Enqueue(&model.WorkItem{
		WorkId: "d2",
		Phase:  v1.WorkPhaseDecode,
	}))
	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  4,
		PrefillTokens: 4,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 3, n)
	require.Len(t, items, 3)
	require.Equal(t, []string{"d1", "d2", "p1"}, []string{
		items[0].WorkId,
		items[1].WorkId,
		items[2].WorkId,
	})
}

func TestPickBatchDoesNotScheduleSmallPrefillThatExceedsRemainingBudget(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 5
	s.maxBatchSeqs = 4

	require.NoError(t, s.decodeQueue.Enqueue(&model.WorkItem{
		WorkId: "d1",
		Phase:  v1.WorkPhaseDecode,
	}))
	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  8,
		PrefillTokens: 8,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 1, n)
	require.Len(t, items, 1)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, uint64(1), s.prefillQueueSmall.Length())
}

func TestPickBatchChunksLargePrefillWithoutRequeueingRemainder(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 10
	s.maxBatchSeqs = 2
	s.maxPartialPrefills = 1
	s.maxLongPartialPrefills = 1

	require.NoError(t, s.prefillQueueLarge.Enqueue(&model.WorkItem{
		WorkId:        "large",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  24,
		PrefillOffset: 0,
		PrefillTokens: 24,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 1, n)
	require.Len(t, items, 1)
	require.Equal(t, "large", items[0].WorkId)
	require.Equal(t, uint64(0), items[0].PrefillOffset)
	require.Equal(t, uint64(10), items[0].PrefillTokens)
	require.Equal(t, uint64(0), s.prefillQueueLarge.Length())
}

func TestPickBatchFillsPrefillOnlyBatchBeyondPartialLimit(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 16
	s.maxBatchSeqs = 8
	s.maxPartialPrefills = 1

	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  2,
		PrefillTokens: 2,
	}))
	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p2",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  2,
		PrefillTokens: 2,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "p1", items[0].WorkId)
	require.Equal(t, "p2", items[1].WorkId)
	require.Equal(t, uint64(0), s.prefillQueueSmall.Length())
}

func TestPickBatchRespectsMaxPartialPrefillsWhenDecodeIsPresent(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 16
	s.maxBatchSeqs = 8
	s.maxPartialPrefills = 1

	require.NoError(t, s.decodeQueue.Enqueue(&model.WorkItem{
		WorkId: "d1",
		Phase:  v1.WorkPhaseDecode,
	}))
	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  2,
		PrefillTokens: 2,
	}))
	require.NoError(t, s.prefillQueueSmall.Enqueue(&model.WorkItem{
		WorkId:        "p2",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  2,
		PrefillTokens: 2,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "p1", items[1].WorkId)
	require.Equal(t, uint64(1), s.prefillQueueSmall.Length())
}

func TestPickBatchFillsPrefillOnlyBatchBeyondLongPartialLimit(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 16
	s.maxBatchSeqs = 8
	s.maxPartialPrefills = 2
	s.maxLongPartialPrefills = 1

	require.NoError(t, s.prefillQueueLarge.Enqueue(&model.WorkItem{
		WorkId:        "l1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  4,
		PrefillTokens: 4,
	}))
	require.NoError(t, s.prefillQueueLarge.Enqueue(&model.WorkItem{
		WorkId:        "l2",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  4,
		PrefillTokens: 4,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "l1", items[0].WorkId)
	require.Equal(t, "l2", items[1].WorkId)
	require.Equal(t, uint64(0), s.prefillQueueLarge.Length())
}

func TestPickBatchRespectsMaxLongPartialPrefillsWhenDecodeIsPresent(t *testing.T) {
	s := newTestScheduler()
	s.maxBatchTokens = 16
	s.maxBatchSeqs = 8
	s.maxPartialPrefills = 2
	s.maxLongPartialPrefills = 1

	require.NoError(t, s.decodeQueue.Enqueue(&model.WorkItem{
		WorkId: "d1",
		Phase:  v1.WorkPhaseDecode,
	}))
	require.NoError(t, s.prefillQueueLarge.Enqueue(&model.WorkItem{
		WorkId:        "l1",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  4,
		PrefillTokens: 4,
	}))
	require.NoError(t, s.prefillQueueLarge.Enqueue(&model.WorkItem{
		WorkId:        "l2",
		Phase:         v1.WorkPhasePrefill,
		PromptTokens:  4,
		PrefillTokens: 4,
	}))

	items, n := requirePickBatchReturns(t, s)

	require.Equal(t, 2, n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "l1", items[1].WorkId)
	require.Equal(t, uint64(1), s.prefillQueueLarge.Length())
}
