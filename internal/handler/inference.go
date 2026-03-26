package handler

import (
	"context"
	"time"

	"github.com/qujing226/mini-llm-serve/internal/errors"
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
		e.Scheduler.Cancel(in.RequestId)
		return nil, errors.Wrap(errors.CodeRequestCanceled, "handler.generate", "request canceled", ctx.Err())
	case <-time.After(in.Timeout):
		e.Scheduler.Cancel(in.RequestId)
		return nil, errors.New(errors.CodeRequestTimeout, "request timeout")
	}

	return res, err
}
