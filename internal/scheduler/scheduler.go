package scheduler

import (
	"context"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/utils"
	"github.com/qujing226/mini-llm-serve/internal/worker"
	"go.uber.org/zap"
)

type Scheduler interface {
	Enqueue(*model.GenerateInput) (chan *model.GenerateOutput, error)
	Batch(ctx context.Context)
	ExecuteNow(*model.GenerateInput) (*model.GenerateOutput, error)
}

type scheduler struct {
	l *zap.SugaredLogger

	queue    Queue
	worker   worker.Executor
	size     int64
	interval time.Duration

	receiveChan chan *model.GenerateOutput
	dispatchMap map[string]chan *model.GenerateOutput
}

func NewScheduler(cfg *conf.Conf, q Queue, worker worker.Executor, l *zap.SugaredLogger) Scheduler {
	s := &scheduler{
		l:           l,
		queue:       q,
		worker:      worker,
		size:        cfg.Server.BatchSize,
		interval:    time.Duration(cfg.Server.BatchTimeout) * time.Millisecond,
		receiveChan: make(chan *model.GenerateOutput, cfg.Server.BatchSize),
	}
	return s
}

func (s *scheduler) Enqueue(input *model.GenerateInput) (chan *model.GenerateOutput, error) {
	task, err := DomainToTask(input)
	if err != nil {
		return nil, err
	}
	err = s.queue.Enqueue(task)
	if err != nil {
		return nil, err
	}
	s.dispatchMap[input.RequestId] = make(chan *model.GenerateOutput)
	return s.dispatchMap[input.RequestId], nil
}

func (s *scheduler) ExecuteNow(input *model.GenerateInput) (*model.GenerateOutput, error) {
	task, err := DomainToTask(input)
	if err != nil {
		return nil, err
	}

	res, err := s.worker.ExecuteOne(task)
	if err != nil {
		return nil, err
	}

	return TaskToDomain(res), nil
}

func (s *scheduler) Batch(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	size := s.size
	go s.dispatch(ctx)
	select {
	case <-ctx.Done():
		return
	case <-ticker.C:
		tasks, err := s.queue.Dequeue(size)
		if err != nil {
			s.l.Errorf("failed to dequeue tasks: %v", err)
		}
		if len(tasks) > 0 {
			bid, err := utils.GenerateUUIDv7()
			if err != nil {
				s.l.Errorf("failed to generate batch id: %v", err)
			}
			go func() {
				taskResList, err := s.worker.Execute(&model.Batch{
					BatchID:   bid,
					BatchSize: int64(len(tasks)),
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
	}

}

func (s *scheduler) dispatch(ctx context.Context) {
	select {
	case <-ctx.Done():
		s.l.Info("context canceled")
		return
	case res := <-s.receiveChan:
		ch := s.dispatchMap[res.RequestId]
		ch <- res
		delete(s.dispatchMap, res.RequestId)
	}
}
