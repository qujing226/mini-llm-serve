package scheduler

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
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

	queue     Queue
	worker    worker.Worker
	batchSize uint64

	queueLength      atomic.Uint64
	inflightRequests atomic.Uint64
	inflightBatches  atomic.Uint64

	ticker *time.Ticker

	receiveChan      chan *model.GenerateOutput
	dispatchMap      map[string]chan *model.GenerateOutput
	patchExecuteChan chan struct{}

	mu      sync.RWMutex
	metrics metrics.Metrics
}

func NewScheduler(cfg *conf.Conf, q Queue, worker worker.Worker, l *zap.SugaredLogger, metrics metrics.Metrics) Scheduler {
	s := &scheduler{
		l:                l,
		queue:            q,
		worker:           worker,
		batchSize:        cfg.Server.BatchSize,
		ticker:           time.NewTicker(cfg.Server.BatchRoundInterval()),
		receiveChan:      make(chan *model.GenerateOutput, cfg.Server.BatchSize),
		dispatchMap:      make(map[string]chan *model.GenerateOutput, cfg.Server.BatchSize*3),
		patchExecuteChan: make(chan struct{}, 1),

		metrics: metrics,
	}
	return s
}

func (s *scheduler) Enqueue(input *model.GenerateInput) (chan *model.GenerateOutput, error) {
	// metrics: set inflight requests
	s.metrics.SetInflightRequests(int(s.inflightRequests.Add(1)))
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
		// metrics: injected request
		s.metrics.IncQueueRejected()
		s.Cancel(input.RequestId)
		return nil, err
	}

	// metrics: queue queueLength
	s.metrics.SetQueueLength(int(s.queue.Length()))

	s.queueLength.Add(1)
	if s.queueLength.CompareAndSwap(s.batchSize, 0) {
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

func (s *scheduler) Cancel(requestId string) {
	s.mu.Lock()
	if _, exist := s.dispatchMap[requestId]; exist {
		delete(s.dispatchMap, requestId)
		// metrics: reduce inflight request
		s.metrics.SetInflightRequests(int(s.inflightRequests.Add(^uint64(0))))
	}
	s.mu.Unlock()
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

			taskResList, err := s.worker.Batch(ctx, &model.Batch{
				BatchID:   bid,
				BatchSize: uint64(len(tasks)),
				CreateAt:  batchCreateAt,
				Tasks:     tasks,
			})

			s.metrics.SetInflightBatches(int(s.inflightBatches.Add(^uint64(0))))

			if err != nil {
				s.l.Errorf("failed to execute batch: %v", err)
			}
			for _, taskRes := range taskResList {
				output := TaskToDomain(taskRes)
				s.receiveChan <- output
				// metrics: observe every task execution cost and executorId
				s.metrics.ObserveExecution(taskRes.ExecutionTime.Seconds(), taskRes.ExecutorId)
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
		case res := <-s.receiveChan:
			s.mu.Lock()
			if ch, exist := s.dispatchMap[res.RequestId]; exist {
				ch <- res
				delete(s.dispatchMap, res.RequestId)
				// metrics: reduce inflight request
				s.metrics.SetInflightRequests(int(s.inflightRequests.Add(^uint64(0))))
			}
			s.mu.Unlock()

		}
	}
}
