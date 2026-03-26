package worker

import (
	"context"

	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Executor interface {
	Execute(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error)
	GetId() string
}

func NewExecutors(cfg *conf.Conf) map[string]Executor {
	mo := newPythonExecutor()

	executors := make(map[string]Executor)
	executors[mo.GetId()] = mo
	return executors
}

type mockPythonExecutor struct {
}

func newPythonExecutor() Executor {
	return &mockPythonExecutor{}
}

func (m *mockPythonExecutor) Execute(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error) {

	return []*model.TaskResult{r}, nil
}

func (m *mockPythonExecutor) GetId() string {
	return "mock"
}

func (m *mockPythonExecutor) ExecuteOne() string {
	//beginExecutionTime := time.Now()
	//select {
	//case <-ctx.Done():
	//	return nil, ctx.Err()
	//case <-time.After(138 * time.Millisecond):
	//}
	//endExecutionTime := time.Now()
	//r := &model.TaskResult{
	//	TaskId:        task.TaskId,
	//	RequestId:     task.RequestId,
	//	ExecutorId:    m.GetId(),
	//	Output:        "to stimulate output, I spent all of my tokens",
	//	FinishReason:  0,
	//	ExecutionTime: endExecutionTime.Sub(beginExecutionTime),
	//	Usage: model.Usage{
	//		InputTokens:  32,
	//		OutputTokens: 123,
	//		TotalTokens:  155,
	//	},
	//	Error:   nil,
	//	BatchID: batchId,
	//	Timing: model.Timing{
	//		QueueMs:     uint32(batchCreateTime.Sub(task.EnqueuedAt).Milliseconds()),
	//		BatchWaitMs: uint32(beginExecutionTime.Sub(batchCreateTime).Milliseconds()),
	//		ExecutionMs: uint32(endExecutionTime.Sub(beginExecutionTime).Milliseconds()),
	//		TotalMs:     uint32(endExecutionTime.Sub(task.EnqueuedAt).Milliseconds()),
	//	},
	//}
	return "mock"
}
