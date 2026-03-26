package scheduler

import (
	"time"

	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/utils"
)

func TaskToDomain(t *model.TaskResult) *model.GenerateOutput {
	o := &model.GenerateOutput{
		RequestId:    t.RequestId,
		Output:       t.Output,
		FinishReason: t.FinishReason,
		Usage:        t.Usage,
		Timing:       t.Timing,
		BatchID:      t.BatchID,
		ExecutorId:   t.ExecutorId,
	}

	return o
}

func DomainToTask(in *model.GenerateInput) (*model.Task, error) {
	tid, err := utils.GenerateUUIDv7()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	t := &model.Task{
		TaskId:     tid,
		RequestId:  in.RequestId,
		Model:      in.Model,
		Prompt:     in.Prompt,
		MaxTokens:  in.MaxTokens,
		Labels:     in.Labels,
		EnqueuedAt: now,
		DeadLine:   now.Add(in.Timeout),
		ResultCh:   make(chan *model.TaskResult),
	}
	return t, nil
}
