package scheduler

import (
	"testing"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
)

func TestQueueEnqueueDequeueFIFO(t *testing.T) {
	q := NewQueue(&conf.Conf{
		Server: conf.ServerConf{
			QueueLength: 3,
		},
	})

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "t1"}))
	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "t2"}))
	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "t3"}))
	require.Equal(t, uint64(3), q.Length())
	require.Equal(t, uint64(0), q.AvailableSpace())

	tasks, err := q.Dequeue(2)
	require.NoError(t, err)
	require.Len(t, tasks, 2)
	require.Equal(t, "t1", tasks[0].WorkId)
	require.Equal(t, "t2", tasks[1].WorkId)
	require.Equal(t, uint64(1), q.Length())
	require.Equal(t, uint64(2), q.AvailableSpace())

	tasks, err = q.Dequeue(2)
	require.NoError(t, err)
	require.Len(t, tasks, 1)
	require.Equal(t, "t3", tasks[0].WorkId)
	require.Equal(t, uint64(0), q.Length())
	require.Equal(t, uint64(3), q.AvailableSpace())
}

func TestQueueFull(t *testing.T) {
	q := NewQueue(&conf.Conf{
		Server: conf.ServerConf{
			QueueLength: 1,
		},
	})

	require.NoError(t, q.Enqueue(&model.WorkItem{WorkId: "t1"}))
	err := q.Enqueue(&model.WorkItem{WorkId: "t2"})
	require.Error(t, err)
}

func TestQueueDequeueEmptyOrZero(t *testing.T) {
	q := NewQueue(&conf.Conf{
		Server: conf.ServerConf{
			QueueLength: 2,
		},
	})

	tasks, err := q.Dequeue(0)
	require.NoError(t, err)
	require.Nil(t, tasks)

	tasks, err = q.Dequeue(1)
	require.NoError(t, err)
	require.Nil(t, tasks)
}
