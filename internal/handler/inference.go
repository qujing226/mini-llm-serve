package handler

import (
	"context"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/scheduler"
	"github.com/qujing226/mini-llm-serve/internal/state"
	"go.uber.org/zap"
)

type InferenceHandler interface {
	Generate(ctx context.Context, req *model.Request) (*model.GenerateOutput, error)
	GenerateStream(ctx context.Context, req *model.Request) (<-chan *model.GenerateOutput, error)
}

type inferenceHandler struct {
	l *zap.SugaredLogger

	Scheduler      scheduler.Scheduler
	RequestManager state.RequestLifecycleStateManager
}

func NewInferenceHandle(l *zap.SugaredLogger, s scheduler.Scheduler, r state.RequestLifecycleStateManager) InferenceHandler {
	e := &inferenceHandler{
		l:              l,
		Scheduler:      s,
		RequestManager: r,
	}
	return e
}

func (e *inferenceHandler) Generate(ctx context.Context, req *model.Request) (*model.GenerateOutput, error) {
	firstWorkItem, err := e.RequestManager.Create(req)
	if err != nil {
		return nil, err
	}

	subscribeChan, err := e.RequestManager.Subscribe(req.RequestId)
	if err != nil {
		e.RequestManager.Cancel(req.RequestId)
		return nil, err
	}

	err = e.Scheduler.Enqueue(firstWorkItem)
	if err != nil {
		e.RequestManager.Cancel(req.RequestId)
		return nil, err
	}
	var res string
	select {
	case event := <-subscribeChan:
		res += event.DeltaText

	case <-ctx.Done():
		e.Scheduler.Cancel(req.RequestId)
		return nil, errors.Wrap(errors.CodeRequestCanceled, "handler.generate", "request canceled", ctx.Err())
	case <-time.After(req.Timeout):
		e.Scheduler.Cancel(req.RequestId)
		return nil, errors.New(errors.CodeRequestTimeout, "request timeout")
	}

	output := &model.GenerateOutput{
		RequestId:    req.RequestId,
		Output:       res,
		FinishReason: v1.FinishReasonStop,
		Usage:        model.Usage{},
		Timing:       model.Timing{},
		BatchID:      "",
		BatchSize:    0,
		ExecutorId:   "",
	}
	return output, err
}

func (e *inferenceHandler) GenerateStream(ctx context.Context, req *model.Request) (<-chan *model.GenerateOutput, error) {
	panic("implement me")
}
