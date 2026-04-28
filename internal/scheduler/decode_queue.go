package scheduler

import (
	"sync"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type DecodeQueue interface {
	Enqueue(t *model.WorkItem) error
	Dequeue(maxSeqs uint64) ([]*model.WorkItem, uint64)
	Length() uint64
	AvailableSpace() uint64
}
type decodeQueue struct {
	mu    sync.Mutex
	works []*workItemEntry
	size  uint64
	round time.Duration
}

type workItemEntry struct {
	work    *model.WorkItem
	deficit uint64
	quant   uint64
}

func NewDecodeQueue(cfg *conf.Conf) DecodeQueue {
	length := cfg.Server.ScheduleConf.QueueConf.QueueLength
	if length == 0 {
		length = 100
	}
	q := &decodeQueue{
		size:  length,
		works: make([]*workItemEntry, 0, length),
		round: cfg.Server.ScheduleConf.QueueConf.QueueRoundInterval(),
	}
	return q
}

func (q *decodeQueue) Enqueue(t *model.WorkItem) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	if uint64(len(q.works)) >= q.size {
		return errors.New(errors.CodeQueueFull, "decodeQueue is full")
	}
	q.works = append(q.works, &workItemEntry{
		work:    t,
		deficit: 0,
		quant:   0,
	})
	return nil
}

func (q *decodeQueue) Dequeue(maxSeqs uint64) ([]*model.WorkItem, uint64) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if maxSeqs == 0 {
		return nil, 0
	}
	workList := make([]*model.WorkItem, 0, maxSeqs)
	for i := uint64(0); i < maxSeqs; i++ {
		if len(q.works) == 0 {
			break
		}
		workList = append(workList, q.works[0].work)
		q.works = q.works[1:]
	}
	return workList, uint64(len(workList))
}

func (q *decodeQueue) Length() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return uint64(len(q.works))
}

func (q *decodeQueue) AvailableSpace() uint64 {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size - uint64(len(q.works))
}
