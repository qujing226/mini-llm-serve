package executor

import (
	"context"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/zap"
)

type Manager interface {
	Consume(ctx context.Context)
	Submit(ctx context.Context, batch *model.Batch) error
	Events() <-chan *model.Event
}

type executorManager struct {
	logger *zap.SugaredLogger

	executors    map[string]Executor
	executorList []string

	batchChan chan *model.Batch
	eventChan chan *model.Event

	metrics metrics.Metrics

	inflightBatches atomic.Uint64
}

func NewExecutorManager(logger *zap.SugaredLogger, executors map[string]Executor, metrics metrics.Metrics) Manager {
	executorNum := len(executors)

	// todo: 调度管理
	executorList := make([]string, executorNum)
	idx := 0
	for s, _ := range executors {
		executorList[idx] = s
		idx++
	}

	e := &executorManager{
		logger:       logger,
		executors:    executors,
		executorList: executorList,
		batchChan:    make(chan *model.Batch, 100),
		eventChan:    make(chan *model.Event, 100),
		metrics:      metrics,
	}
	return e
}

func (e *executorManager) Submit(ctx context.Context, batch *model.Batch) error {
	select {
	case e.batchChan <- batch:
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New(errors.CodeQueueFull, "executor batch queue is full")
	}
	return nil
}

func (e *executorManager) Events() <-chan *model.Event {
	return e.eventChan
}

func (e *executorManager) Consume(ctx context.Context) {
	for _, executorId := range e.executorList {
		go e.consumeExecutor(ctx, executorId, e.executors[executorId])
	}
	<-ctx.Done()
}

func (e *executorManager) consumeExecutor(ctx context.Context, executorId string, executor Executor) {
	for {
		select {
		case <-ctx.Done():
			return
		case batch, ok := <-e.batchChan:
			if !ok {
				return
			}

			e.metrics.SetInflightBatches(int(e.inflightBatches.Add(1)))
			e.metrics.IncBatches(executorId)
			events, err := executor.Execute(ctx, batch)
			if err != nil {
				e.metrics.IncExecutorErrors(executorId)
				if len(events) == 0 {
					events = batchFailedEvents(batch, executorId, err)
				} else {
					markEventsFailed(events, err)
				}
			}
			e.metrics.SetInflightBatches(int(e.inflightBatches.Add(^uint64(0))))

			for _, event := range events {
				select {
				case e.eventChan <- event:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func markEventsFailed(events []*model.Event, err error) {
	for _, event := range events {
		event.Err = err
		event.Done = true
		event.FinishReason = v1.FinishReasonError
		event.Type = v1.EventTypeRequestFailed
	}
}

func batchFailedEvents(batch *model.Batch, executorId string, err error) []*model.Event {
	events := make([]*model.Event, 0, len(batch.Items))
	for _, item := range batch.Items {
		events = append(events, &model.Event{
			WorkId:       item.WorkId,
			RequestId:    item.RequestId,
			BatchId:      batch.BatchID,
			ExecutorId:   executorId,
			Type:         v1.EventTypeRequestFailed,
			Done:         true,
			FinishReason: v1.FinishReasonError,
			At:           time.Now(),
			Err:          err,
		})
	}
	return events
}
