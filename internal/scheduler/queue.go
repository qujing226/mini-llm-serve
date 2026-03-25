package scheduler

import (
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Queue interface {
	Enqueue(t *model.Task) error
	Dequeue(n int64) ([]*model.Task, error)
	Length() int64
}

type queue struct {
	queue []*model.Task
	round time.Duration
	size  int64
}

func NewQueue(cfg *conf.Conf) Queue {
	length := cfg.Server.QueueLength
	if length < 8 || length > 1000 {
		length = 100
	}
	q := &queue{
		size:  length,
		queue: make([]*model.Task, length),
		round: time.Duration(cfg.Server.QueueRoundTime) * time.Millisecond,
	}
	return q
}

// todo: 使用环型队列

func (q *queue) Enqueue(task *model.Task) error {
	q.queue[q.Length()%q.size] = task
	return nil
}

func (q *queue) Dequeue(n int64) ([]*model.Task, error) {
	taskList := q.queue[:n]
	q.queue = q.queue[n:]
	return taskList, nil
}

func (q *queue) Length() int64 {
	return int64(len(q.queue))
}
