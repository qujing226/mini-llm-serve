package handler

import (
	"context"

	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Engine interface {
	Generate(ctx context.Context, in *model.GenerateInput) (*model.GenerateOutput, error)
}

type EngineHandle struct {
}

func NewEngineHandle() *EngineHandle {
	e := &EngineHandle{}
	return e
}

func (e *EngineHandle) Generate(ctx context.Context, in *model.GenerateInput) (*model.GenerateOutput, error) {
	//TODO implement me
	panic("implement me")
}
