package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"github.com/qujing226/mini-llm-serve/internal/utils"
	"github.com/qujing226/mini-llm-serve/internal/worker"
	"go.uber.org/zap"
)

type Scheduler interface {
	Enqueue(input *model.WorkItem) error
	Batch(ctx context.Context)
	ExecuteNow(ctx context.Context, input *model.WorkItem) error
}

type scheduler struct {
	l *zap.SugaredLogger

	queue          Queue
	requestManager state.RequestLifecycleStateManager
	worker         worker.Worker
	batchSize      uint64

	queueLength      atomic.Uint64
	inflightRequests atomic.Uint64
	inflightBatches  atomic.Uint64

	ticker *time.Ticker

	patchExecuteChan chan struct{}

	mu      sync.RWMutex
	metrics metrics.Metrics
}

func NewScheduler(l *zap.SugaredLogger, cfg *conf.Conf, q Queue, worker worker.Worker, r state.RequestLifecycleStateManager, metrics metrics.Metrics) Scheduler {
	s := &scheduler{
		l:                l,
		queue:            q,
		worker:           worker,
		requestManager:   r,
		batchSize:        cfg.Server.BatchSize,
		ticker:           time.NewTicker(cfg.Server.BatchRoundInterval()),
		patchExecuteChan: make(chan struct{}, 1),

		metrics: metrics,
	}
	return s
}

func (s *scheduler) Enqueue(input *model.WorkItem) error {
	// metrics: set inflight requests
	s.metrics.SetInflightRequests(int(s.inflightRequests.Add(1)))

	err := s.queue.Enqueue(input)
	if err != nil {
		// metrics: injected request
		s.metrics.IncQueueRejected()
		return err
	}

	// metrics: queue queueLength
	s.metrics.SetQueueLength(int(s.queue.Length()))

	s.queueLength.Add(1)
	if s.queueLength.CompareAndSwap(s.batchSize, 0) {
		s.patchExecuteChan <- struct{}{}
	}
	return nil
}

func (s *scheduler) ExecuteNow(ctx context.Context, input *model.WorkItem) error {
	panic("implement me")
	//task, err := WorkItemToTask(input)
	//if err != nil {
	//	return nil, err
	//}
	//
	//res, err := s.worker.One(ctx, task)
	//if err != nil {
	//	return nil, err
	//}
	//
	//return TaskToDomain(res), nil
}

func (s *scheduler) Batch(ctx context.Context) {
	defer s.ticker.Stop()
	go s.dispatch(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ticker.C:
			s.patchExecute(ctx)
		case <-s.patchExecuteChan:
			s.patchExecute(ctx)
		}
	}
}

func (s *scheduler) patchExecute(ctx context.Context) {
	tasks, err := s.queue.Dequeue(s.batchSize)
	// metrics: update queue queueLength
	s.metrics.SetQueueLength(int(s.queue.Length()))

	if err != nil {
		s.l.Errorf("failed to dequeue tasks: %v", err)
	}
	taskLength := len(tasks)
	if taskLength > 0 {
		// metrics: observe batch batchSize & inflight batch number
		s.metrics.ObserveBatchSize(taskLength)
		s.metrics.SetInflightBatches(int(s.inflightBatches.Add(1)))

		bid, err := utils.GenerateUUIDv7()
		if err != nil {
			s.l.Errorf("failed to generate batch id: %v", err)
		}
		go func() {
			batchCreateAt := time.Now()
			// metrics: observe queue wait ms
			for _, task := range tasks {
				s.metrics.ObserveQueueWait(batchCreateAt.Sub(task.EnqueuedAt).Seconds())
			}

			eventList, err := s.worker.Batch(ctx, &model.Batch{
				BatchID:   bid,
				BatchSize: uint64(len(tasks)),
				CreateAt:  batchCreateAt,
				Items:     tasks,
			})

			s.metrics.SetInflightBatches(int(s.inflightBatches.Add(^uint64(0))))

			if err != nil {
				s.l.Errorf("failed to execute batch: %v", err)
			}
			for _, event := range eventList {
				nextItems, err := s.requestManager.OnEvent(event)
				if err != nil {
					s.l.Errorf("failed to execute event: %v", err)
				}
				for _, nextItem := range nextItems {
					err = s.queue.Enqueue(nextItem)
				}

				// metrics: observe every task execution cost and executorId
				s.metrics.ObserveExecution(event.Timing.Execution.Seconds(), event.ExecutorId)
			}
		}()
	}
	s.queueLength.Add(-uint64(taskLength))
}

func (s *scheduler) dispatch(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			s.l.Info("context canceled")
			return
		}
	}
}
