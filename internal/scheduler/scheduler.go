package scheduler

import "github.com/qujing226/mini-llm-serve/internal/model"

type Scheduler interface {
	Enqueue(*model.Task) error
	Batch() (*model.Batch, error)
}

type scheduler struct {
	queue Queue
}

func NewScheduler(q Queue) Scheduler {
	s := &scheduler{
		queue: q,
	}
	return s
}

func (s *scheduler) Enqueue(task *model.Task) error {
	//TODO implement me
	panic("implement me")
}

func (s *scheduler) Batch() (*model.Batch, error) {
	//TODO implement me
	panic("implement me")
}
