package executor

import "github.com/qujing226/mini-llm-serve/internal/model"

type Executor interface {
	Execute(batch *model.Batch) ([]*model.TaskResult, error)
}

type executor struct {
}

func NewExecutor() Executor {
	e := &executor{}
	return e
}

func (e *executor) Execute(batch *model.Batch) ([]*model.TaskResult, error) {
	panic("implement me")
}
