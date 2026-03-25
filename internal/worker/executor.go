package executor

import "github.com/qujing226/mini-llm-serve/internal/model"

type Executor interface {
	Execute(batch *model.Batch) error
}
