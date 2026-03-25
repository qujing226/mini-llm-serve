package handler

import (
	"context"

	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/scheduler"
)

type InferenceHandler interface {
	Generate(ctx context.Context, in *model.GenerateInput) (*model.GenerateOutput, error)
}

type inferenceHandler struct {
	Scheduler scheduler.Scheduler
}

func NewInferenceHandle(s scheduler.Scheduler) InferenceHandler {
	e := &inferenceHandler{
		Scheduler: s,
	}
	return e
}

func (e *inferenceHandler) Generate(ctx context.Context, in *model.GenerateInput) (*model.GenerateOutput, error) {
	return &model.GenerateOutput{
		RequestId:    "",
		Output:       "",
		FinishReason: 0,
		Usage:        model.Usage{},
		Timing:       model.Timing{},
		BatchID:      "",
		BatchSize:    0,
		WorkerId:     "",
	}, nil
}
