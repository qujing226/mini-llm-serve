package scheduler

import (
	"testing"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
)

func testQueueConf(length uint64) *conf.Conf {
	return &conf.Conf{
		Server: conf.ServerConf{
			ScheduleConf: conf.ScheduleConf{
				QueueConf: conf.QueueConf{
					QueueLength: length,
				},
			},
		},
	}
}

func TestPrefillQueuePeekDoesNotRemove(t *testing.T) {
	q := NewPrefillQueue(testQueueConf(3))

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "p1"}))
	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "p2"}))

	item, ok := q.Peek()
	require.True(t, ok)
	require.Equal(t, "p1", item.WorkId)
	require.Equal(t, uint64(2), q.Length())

	item, ok = q.Peek()
	require.True(t, ok)
	require.Equal(t, "p1", item.WorkId)
	require.Equal(t, uint64(2), q.Length())
}

func TestPrefillQueuePopRemovesFIFO(t *testing.T) {
	q := NewPrefillQueue(testQueueConf(3))

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "p1"}))
	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "p2"}))

	item, ok := q.Pop()
	require.True(t, ok)
	require.Equal(t, "p1", item.WorkId)
	require.Equal(t, uint64(1), q.Length())
	require.Equal(t, uint64(2), q.AvailableSpace())

	item, ok = q.Pop()
	require.True(t, ok)
	require.Equal(t, "p2", item.WorkId)
	require.Equal(t, uint64(0), q.Length())
	require.Equal(t, uint64(3), q.AvailableSpace())
}

func TestPrefillQueueEmptyPeekAndPop(t *testing.T) {
	q := NewPrefillQueue(testQueueConf(2))

	item, ok := q.Peek()
	require.False(t, ok)
	require.Nil(t, item)

	item, ok = q.Pop()
	require.False(t, ok)
	require.Nil(t, item)
}

func TestPrefillQueueFull(t *testing.T) {
	q := NewPrefillQueue(testQueueConf(1))

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "p1"}))
	require.Error(t, q.Enqueue(&model.WorkItem{WorkId: "p2"}))
	require.Equal(t, uint64(1), q.Length())
	require.Equal(t, uint64(0), q.AvailableSpace())
}

func TestDecodeQueueDequeueRespectsMaxSeqs(t *testing.T) {
	q := NewDecodeQueue(testQueueConf(3))

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "d1"}))
	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "d2"}))
	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "d3"}))

	items, n := q.Dequeue(2)
	require.Equal(t, uint64(2), n)
	require.Len(t, items, 2)
	require.Equal(t, "d1", items[0].WorkId)
	require.Equal(t, "d2", items[1].WorkId)
	require.Equal(t, uint64(1), q.Length())

	items, n = q.Dequeue(2)
	require.Equal(t, uint64(1), n)
	require.Len(t, items, 1)
	require.Equal(t, "d3", items[0].WorkId)
	require.Equal(t, uint64(0), q.Length())
}

func TestDecodeQueueDequeueZeroOrEmpty(t *testing.T) {
	q := NewDecodeQueue(testQueueConf(2))

	items, n := q.Dequeue(0)
	require.Equal(t, uint64(0), n)
	require.Nil(t, items)

	items, n = q.Dequeue(1)
	require.Equal(t, uint64(0), n)
	require.Empty(t, items)
}

func TestDecodeQueueFull(t *testing.T) {
	q := NewDecodeQueue(testQueueConf(1))

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "d1"}))
	require.Error(t, q.Enqueue(&model.WorkItem{WorkId: "d2"}))
	require.Equal(t, uint64(1), q.Length())
	require.Equal(t, uint64(0), q.AvailableSpace())
}
