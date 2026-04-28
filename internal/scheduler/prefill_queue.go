package scheduler

import (
	"sync"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type PrefillQueue interface {
	Enqueue(t *model.WorkItem) error
	Pop() (*model.WorkItem, bool)
	Peek() (*model.WorkItem, bool)
	Length() uint64
	AvailableSpace() uint64
}

type prefillQueue struct {
	mu    sync.Mutex
	works []*model.WorkItem
	size  uint64
	round time.Duration
}

func NewPrefillQueue(cfg *conf.Conf) PrefillQueue {
	length := cfg.Server.ScheduleConf.QueueConf.QueueLength
	if length == 0 {
		length = 100
	}
	q := &prefillQueue{
		size:  length,
		works: make([]*model.WorkItem, 0, length),
		round: cfg.Server.ScheduleConf.QueueConf.QueueRoundInterval(),
	}
	return q
}

func (q *prefillQueue) Enqueue(t *model.WorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uint64(len(q.works)) >= q.size {
		return errors.New(errors.CodeQueueFull, "prefillQueue is full")
	}
	q.works = append(q.works, t)
	return nil
}

func (q *prefillQueue) Pop() (*model.WorkItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.works) == 0 {
		return nil, false
	}
	work := q.works[0]
	q.works = q.works[1:]
	return work, true
}

func (q *prefillQueue) Peek() (*model.WorkItem, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.works) == 0 {
		return nil, false
	}
	return q.works[0], true
}

func (q *prefillQueue) Dequeue(tokens uint64) ([]*model.WorkItem, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if tokens == 0 || len(q.works) == 0 {
		return nil, nil
	}
	workList := make([]*model.WorkItem, 0, 10)
	used := uint64(0)

	for len(q.works) > 0 {
		cost := WorkBudgetCost(q.works[0])
		if used+cost > tokens {
			break
		}
		workList = append(workList, q.works[0])
		q.works = q.works[1:]
		used += cost
	}
	if len(workList) == 0 {
		workList = append(workList, q.works[0])
		q.works = q.works[1:]
	}
	return workList, nil
}

func (q *prefillQueue) Length() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint64(len(q.works))
}

func (q *prefillQueue) AvailableSpace() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size - uint64(len(q.works))
}
