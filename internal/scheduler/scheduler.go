package scheduler

import (
	"context"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
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
}

type scheduler struct {
	l *zap.SugaredLogger

	prefillQueue   Queue
	decodeQueue    Queue
	requestManager state.RequestLifecycleStateManager
	worker         worker.Worker
	batchSize      uint64

	inflightRequests atomic.Uint64
	inflightBatches  atomic.Uint64

	ticker *time.Ticker

	prefillPatchExecuteChan chan struct{}
	decodePatchExecuteChan  chan struct{}

	metrics metrics.Metrics
}

func NewScheduler(l *zap.SugaredLogger, cfg *conf.Conf, prefillQ Queue, decodeQ Queue, worker worker.Worker, r state.RequestLifecycleStateManager, metrics metrics.Metrics) Scheduler {
	s := &scheduler{
		l:                       l,
		prefillQueue:            prefillQ,
		decodeQueue:             decodeQ,
		worker:                  worker,
		requestManager:          r,
		batchSize:               cfg.Server.BatchSize,
		ticker:                  time.NewTicker(cfg.Server.BatchRoundInterval()),
		prefillPatchExecuteChan: make(chan struct{}, 1),
		decodePatchExecuteChan:  make(chan struct{}, 1),

		metrics: metrics,
	}
	return s
}

func (s *scheduler) Enqueue(input *model.WorkItem) error {
	input.EnqueuedAt = time.Now()
	var err error
	switch input.Phase {
	case v1.WorkPhasePrefill:
		err = s.prefillQueue.Enqueue(input)
		if err == nil {
			// metrics: set inflight requests
			s.metrics.SetInflightRequests(int(s.inflightRequests.Add(1)))
			// metrics: prefillQueue queueLength
			s.metrics.SetPrefillQueueLength(int(s.prefillQueue.Length()))
			if s.prefillQueue.Length() >= s.batchSize {
				s.signal(s.prefillPatchExecuteChan)
			}
		}
	case v1.WorkPhaseDecode:
		err = s.decodeQueue.Enqueue(input)
		if err == nil {
			s.metrics.SetDecodeQueueLength(int(s.decodeQueue.Length()))
			if s.decodeQueue.Length() >= s.batchSize {
				s.signal(s.decodePatchExecuteChan)
			}
		}
	default:
		return errors.New(errors.CodeInvalidArgument, "invalid phase for enqueue")
	}

	if err != nil {
		// metrics: injected request
		s.metrics.IncQueueRejected()
		s.requestManager.Fail(input.RequestId, err)
		s.l.Errorw("enqueue failed", "phase", input.Phase, "error", err)
		return err
	}

	return nil
}

func (s *scheduler) Batch(ctx context.Context) {
	defer s.ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.ticker.C:
			s.patchExecute(ctx, v1.WorkPhaseDecode)
			s.patchExecute(ctx, v1.WorkPhasePrefill)
		case <-s.prefillPatchExecuteChan:
			s.patchExecute(ctx, v1.WorkPhasePrefill)
		case <-s.decodePatchExecuteChan:
			s.patchExecute(ctx, v1.WorkPhaseDecode)
		}
	}
}

func (s *scheduler) patchExecute(ctx context.Context, phase v1.WorkPhase) {
	var workItems []*model.WorkItem
	var err error
	switch phase {
	case v1.WorkPhasePrefill:
		workItems, err = s.prefillQueue.Dequeue(s.batchSize)
		s.metrics.SetPrefillQueueLength(int(s.prefillQueue.Length()))
	case v1.WorkPhaseDecode:
		workItems, err = s.decodeQueue.Dequeue(s.batchSize)
		s.metrics.SetDecodeQueueLength(int(s.decodeQueue.Length()))
	default:
		s.l.Errorw("invalid work phase", "phase", phase)
		return
	}
	if err != nil {
		s.l.Errorf("failed to dequeue workItems: %v", err)
		return
	}

	taskLength := len(workItems)
	if taskLength <= 0 {
		return
	}

	// metrics: observe batch batchSize & inflight batch number
	s.metrics.ObserveBatchSize(taskLength, phase)
	s.metrics.SetInflightBatches(int(s.inflightBatches.Add(1)))

	bid, err := utils.GenerateUUIDv7()
	if err != nil {
		s.l.Errorf("failed to generate batch id: %v", err)
	}
	go func() {
		batchCreateAt := time.Now()
		// metrics: observe prefillQueue wait ms
		for _, task := range workItems {
			s.metrics.ObserveQueueWait(batchCreateAt.Sub(task.EnqueuedAt).Seconds())
		}

		eventList, err := s.worker.Batch(ctx, &model.Batch{
			BatchID:   bid,
			BatchSize: uint64(len(workItems)),
			Phase:     phase,
			CreateAt:  batchCreateAt,
			Items:     workItems,
		})

		s.metrics.SetInflightBatches(int(s.inflightBatches.Add(^uint64(0))))

		if err != nil {
			for _, e := range workItems {
				s.l.Errorf("failed to inflight task: %v", e)
				s.requestManager.Fail(e.RequestId, err)
			}
			s.l.Errorf("failed to execute batch: %v, batchId: %s", err, bid)
			return
		}
		for _, event := range eventList {
			// generated next task
			nextItems, err := s.requestManager.OnEvent(event)
			if err != nil {
				s.l.Errorf("failed to execute event: %v", err)
			}
			for _, nextItem := range nextItems {
				err = s.Enqueue(nextItem)
			}

			// metrics: observe every task execution cost and executorId
			s.metrics.ObserveExecution(event.Timing.Execution.Seconds(), event.ExecutorId)
		}
	}()
}

func (s *scheduler) signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}
