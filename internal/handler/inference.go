package handler

import (
	"context"
	"time"

	"github.com/cockroachdb/errors"
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
	ch, err := e.Scheduler.Enqueue(in)
	if err != nil {
		return nil, err
	}
	var res *model.GenerateOutput
	select {
	case res = <-ch:

	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(30 * time.Second):
		return nil, errors.New("timeout")
	}

	return res, err
}
