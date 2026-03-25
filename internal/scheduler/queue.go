package scheduler

import (
	"time"

	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Queue interface {
	Enqueue(t *model.Task) error
	Dequeue() (*model.Task, error)
}

type queue struct {
	round time.Time
}

func NewQueue(round time.Time) Queue {
	q := &queue{
		round: round,
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
