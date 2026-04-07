package scheduler

import (
	"sync"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Queue interface {
	Enqueue(t *model.Task) error
	Dequeue(n uint64) ([]*model.Task, error)
	Length() uint64
	AvailableSpace() uint64
}

type queue struct {
	mu    sync.Mutex
	tasks []*model.Task
	size  uint64
	round time.Duration
}

func NewQueue(cfg *conf.Conf) Queue {
	length := cfg.Server.QueueLength
	if length == 0 {
		length = 100
	}
	q := &queue{
		size:  length,
		tasks: make([]*model.Task, 0, length),
		round: cfg.Server.QueueRoundInterval(),
	}
	return q
}

func (q *queue) Enqueue(t *model.Task) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uint64(len(q.tasks)) >= q.size {
		return errors.New(errors.CodeQueueFull, "queue is full")
	}
	q.tasks = append(q.tasks, t)
	return nil
}

func (q *queue) Dequeue(n uint64) ([]*model.Task, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if n == 0 || len(q.tasks) == 0 {
		return nil, nil
	}
	if n > uint64(len(q.tasks)) {
		n = uint64(len(q.tasks))
	}

	taskList := append([]*model.Task(nil), q.tasks[:n]...)
	q.tasks = q.tasks[n:]
	return taskList, nil
}

func (q *queue) Length() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint64(len(q.tasks))
}

func (q *queue) AvailableSpace() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size - uint64(len(q.tasks))
}
