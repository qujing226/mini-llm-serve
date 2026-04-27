package scheduler

import (
	"sync"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Queue interface {
	Enqueue(t *model.WorkItem) error
	Dequeue(n uint64) ([]*model.WorkItem, error)
	Length() uint64
	AvailableSpace() uint64
}

type prefillQueue struct {
	mu    sync.Mutex
	tasks []*model.WorkItem
	size  uint64
	round time.Duration
}

func NewPrefillQueue(cfg *conf.Conf) Queue {
	length := cfg.Server.QueueLength
	if length == 0 {
		length = 100
	}
	q := &prefillQueue{
		size:  length,
		tasks: make([]*model.WorkItem, 0, length),
		round: cfg.Server.QueueRoundInterval(),
	}
	return q
}

func (q *prefillQueue) Enqueue(t *model.WorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uint64(len(q.tasks)) >= q.size {
		return errors.New(errors.CodeQueueFull, "prefillQueue is full")
	}
	q.tasks = append(q.tasks, t)
	return nil
}

func (q *prefillQueue) Dequeue(n uint64) ([]*model.WorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if n == 0 || len(q.tasks) == 0 {
		return nil, nil
	}
	if n > uint64(len(q.tasks)) {
		n = uint64(len(q.tasks))
	}

	taskList := append([]*model.WorkItem(nil), q.tasks[:n]...)
	q.tasks = q.tasks[n:]
	return taskList, nil
}

func (q *prefillQueue) Length() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint64(len(q.tasks))
}

func (q *prefillQueue) AvailableSpace() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size - uint64(len(q.tasks))
}
