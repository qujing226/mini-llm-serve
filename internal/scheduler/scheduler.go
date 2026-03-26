package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/utils"
	"github.com/qujing226/mini-llm-serve/internal/worker"
	"go.uber.org/zap"
)

type Scheduler interface {
	Enqueue(input *model.GenerateInput) (chan *model.GenerateOutput, error)
	Batch(ctx context.Context)
	ExecuteNow(ctx context.Context, input *model.GenerateInput) (*model.GenerateOutput, error)
	Cancel(requestId string)
}

type scheduler struct {
	l *zap.SugaredLogger

	queue    Queue
	worker   worker.Worker
	size     uint64
	length   atomic.Uint64
	interval time.Duration

	receiveChan      chan *model.GenerateOutput
	dispatchMap      map[string]chan *model.GenerateOutput
	patchExecuteChan chan struct{}

	mu sync.RWMutex
}

func NewScheduler(cfg *conf.Conf, q Queue, worker worker.Worker, l *zap.SugaredLogger) Scheduler {
	s := &scheduler{
		l:                l,
		queue:            q,
		worker:           worker,
		size:             cfg.Server.BatchSize,
		interval:         time.Duration(cfg.Server.BatchTimeout) * time.Millisecond,
		receiveChan:      make(chan *model.GenerateOutput, cfg.Server.BatchSize),
		dispatchMap:      make(map[string]chan *model.GenerateOutput, cfg.Server.BatchSize*3),
		patchExecuteChan: make(chan struct{}, 1),
	}
	return s
}

func (s *scheduler) Enqueue(input *model.GenerateInput) (chan *model.GenerateOutput, error) {
	task, err := DomainToTask(input)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	resCh := make(chan *model.GenerateOutput, 1)
	s.dispatchMap[input.RequestId] = resCh
	s.mu.Unlock()
	err = s.queue.Enqueue(task)
	if err != nil {
		s.Cancel(input.RequestId)
		return nil, err
	}

	s.length.Add(1)
	if s.length.CompareAndSwap(s.size, 0) {
		s.patchExecuteChan <- struct{}{}
	}
	return resCh, nil
}

func (s *scheduler) ExecuteNow(ctx context.Context, input *model.GenerateInput) (*model.GenerateOutput, error) {
	task, err := DomainToTask(input)
	if err != nil {
		return nil, err
	}

	res, err := s.worker.One(ctx, task)
	if err != nil {
		return nil, err
	}

	return TaskToDomain(res), nil
}

func (s *scheduler) Batch(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	go s.dispatch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.patchExecute(ctx)
		case <-s.patchExecuteChan:
			s.patchExecute(ctx)
		}
	}
}

func (s *scheduler) Cancel(requestId string) {
	s.mu.Lock()
	if _, exist := s.dispatchMap[requestId]; exist {
		delete(s.dispatchMap, requestId)
	}
	s.mu.Unlock()
}

func (s *scheduler) patchExecute(ctx context.Context) {
	tasks, err := s.queue.Dequeue(s.size)
	if err != nil {
		s.l.Errorf("failed to dequeue tasks: %v", err)
	}
	taskLength := len(tasks)
	if taskLength > 0 {
		bid, err := utils.GenerateUUIDv7()
		if err != nil {
			s.l.Errorf("failed to generate batch id: %v", err)
		}
		go func() {
			taskResList, err := s.worker.Batch(ctx, &model.Batch{
				BatchID:   bid,
				BatchSize: int64(len(tasks)),
				CreateAt:  time.Now(),
				Tasks:     tasks,
			})
			if err != nil {
				s.l.Errorf("failed to execute batch: %v", err)
			}
			for _, taskRes := range taskResList {
				output := TaskToDomain(taskRes)
				s.receiveChan <- output
			}
		}()
	}
	s.length.Add(-uint64(taskLength))
}

func (s *scheduler) dispatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.l.Info("context canceled")
			return
		case res := <-s.receiveChan:
			s.mu.Lock()
			if ch, exist := s.dispatchMap[res.RequestId]; exist {
				ch <- res
				delete(s.dispatchMap, res.RequestId)
			}
			s.mu.Unlock()

		}
	}
}
