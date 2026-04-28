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

	maxBatchSeqs           uint64
	maxBatchTokens         uint64
	maxPartialPrefills     uint64
	maxLongPartialPrefills uint64
	longPrefillThreshold   uint64

	prefillQueueSmall PrefillQueue
	prefillQueueLarge PrefillQueue
	decodeQueue       DecodeQueue

	requestManager state.RequestLifecycleStateManager
	worker         worker.Worker

	inflightRequests atomic.Uint64
	inflightBatches  atomic.Uint64

	ticker *time.Ticker

	prefillPatchExecuteChan chan struct{}
	decodePatchExecuteChan  chan struct{}

	metrics metrics.Metrics
}

func NewScheduler(l *zap.SugaredLogger, cfg *conf.Conf, prefillQS PrefillQueue, prefillQL PrefillQueue, decodeQ DecodeQueue, worker worker.Worker, r state.RequestLifecycleStateManager, metrics metrics.Metrics) Scheduler {
	s := &scheduler{
		l:                      l,
		maxBatchSeqs:           cfg.Server.ScheduleConf.MaxBatchSeq,
		maxBatchTokens:         cfg.Server.ScheduleConf.MaxBatchTokens,
		maxPartialPrefills:     cfg.Server.ScheduleConf.MaxPartialPrefills,
		maxLongPartialPrefills: cfg.Server.ScheduleConf.MaxLongPartialPrefills,
		longPrefillThreshold:   cfg.Server.ScheduleConf.LongPrefillTokenThreshold,

		prefillQueueSmall:       prefillQS,
		prefillQueueLarge:       prefillQL,
		decodeQueue:             decodeQ,
		worker:                  worker,
		requestManager:          r,
		ticker:                  time.NewTicker(cfg.Server.ScheduleConf.BatchRoundInterval()),
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
		if input.PromptTokens <= s.longPrefillThreshold {
			err = s.prefillQueueSmall.Enqueue(input)
		} else {
			err = s.prefillQueueLarge.Enqueue(input)
		}
		if err == nil {
			// metrics: set inflight requests
			s.metrics.SetInflightRequests(int(s.inflightRequests.Add(1)))
			// metrics: prefillQueue queueLength
			queueLength := s.prefillQueueSmall.Length() + s.prefillQueueLarge.Length()
			s.metrics.SetPrefillQueueLength(int(queueLength))
			if queueLength >= s.maxBatchSeqs {
				s.signal(s.prefillPatchExecuteChan)
			}
		}
	case v1.WorkPhaseDecode:
		err = s.decodeQueue.Enqueue(input)
		if err == nil {
			s.metrics.SetDecodeQueueLength(int(s.decodeQueue.Length()))
			if s.decodeQueue.Length() >= s.maxBatchSeqs {
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
			s.patchExecute(ctx)
			s.patchExecute(ctx)
		case <-s.prefillPatchExecuteChan:
			s.patchExecute(ctx)
		case <-s.decodePatchExecuteChan:
			s.patchExecute(ctx)
		}
	}
}

func (s *scheduler) patchExecute(ctx context.Context) {
	var (
		err error
	)

	workItems, taskLength := s.pickBatch()
	if taskLength <= 0 {
		return
	}

	// metrics: observe batch batchSize & inflight batch number
	s.metrics.ObserveBatchSize(taskLength)
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

func (s *scheduler) pickBatch() ([]*model.WorkItem, int) {
	remainTokens := s.maxBatchTokens
	remainSeqs := s.maxBatchSeqs
	remainPrefill := s.maxPartialPrefills
	remainLongPrefill := s.maxLongPartialPrefills

	batch := make([]*model.WorkItem, 0, remainSeqs)
	items, itemNums := s.decodeQueue.Dequeue(min(remainSeqs, remainTokens))
	if itemNums != 0 {
		remainTokens -= itemNums
		remainSeqs -= itemNums
		batch = append(batch, items...)
	} else {
		remainPrefill, remainLongPrefill = remainSeqs, remainSeqs
	}

	for remainSeqs > 0 && remainTokens > 0 && remainPrefill > 0 {
		if small, ok := s.prefillQueueSmall.Peek(); ok {
			cost := WorkBudgetCost(small)
			if cost <= remainTokens {
				small, ok = s.prefillQueueSmall.Pop()
				if !ok {
					continue
				}
				batch = append(batch, small)
				remainTokens -= cost
				remainSeqs--
				remainPrefill--
				continue
			}
		}
		if remainLongPrefill <= 0 {
			break
		}
		if large, ok := s.prefillQueueLarge.Peek(); ok {
			large, ok = s.prefillQueueLarge.Pop()
			if !ok {
				continue
			}
			cost := WorkBudgetCost(large)
			scheduledTokens := min(cost, remainTokens)
			chunk, _ := splitPrefillChunk(large, scheduledTokens)

			batch = append(batch, chunk)
			remainTokens -= scheduledTokens
			remainSeqs--
			remainPrefill--
			remainLongPrefill--
			continue
		}
		break
	}

	return batch, len(batch)
}

func (s *scheduler) signal(c chan struct{}) {
	select {
	case c <- struct{}{}:
	default:
	}
}
