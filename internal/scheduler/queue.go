package scheduler

import (
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Queue interface {
	Enqueue(t *model.Task) error
	Dequeue() (*model.Task, error)
}

type queue struct {
	round time.Duration
}

func NewQueue(cfg *conf.Conf) Queue {
	q := &queue{
		round: time.Duration(cfg.Server.QueueRoundTime) * time.Millisecond,
	}
	return q
}

func (q *queue) Enqueue(t *model.Task) error {
	//TODO implement me
	panic("implement me")
}

func (q *queue) Dequeue() (*model.Task, error) {
	//TODO implement me
	panic("implement me")
}
