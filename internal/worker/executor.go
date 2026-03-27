package worker

import (
	"context"
	"fmt"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/conf"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Executor interface {
	Execute(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error)
	ExecuteToBatch(batchId string, resp *v1.ExecuteBatchResponse) ([]*model.TaskResult, error)
}

func NewExecutors(cfg *conf.Conf) (map[string]Executor, error) {
	executors := make(map[string]Executor)

	for _, ec := range cfg.Executors {
		if !ec.Enabled {
			continue
		}
		if ec.ID == "" {
			return nil, fmt.Errorf("executor.id can not be empty")
		}
		if ec.Kind == "" {
			return nil, fmt.Errorf("executor.kind can not be empty")
		}
		if len(ec.Address) == 0 {
			return nil, fmt.Errorf("executor.address can not be empty")
		}

		var exec Executor
		var err error
		switch ec.Kind {
		case "connect":
			exec, err = newPythonExecutor(ec)
		case "http":
			exec, err = newHTTPExecutor(ec)
		default:
			return nil, fmt.Errorf("unsupported executor.kind: %s", ec.Kind)
		}
		if err != nil {
			return nil, err
		}
		if _, exists := executors[ec.ID]; exists {
			return nil, fmt.Errorf("executor with id %s already exists", ec.ID)
		}
		executors[ec.ID] = exec
	}
	if len(executors) == 0 {
		return nil, fmt.Errorf("no executors configured")
	}

	return executors, nil
}

type mockExecutor struct {
	id        string
	endpoints []string
	client    *client.ExecutorClient
}

func newPythonExecutor(cfg conf.ExecutorConf) (Executor, error) {
	e := &mockExecutor{
		id:        cfg.ID,
		endpoints: cfg.Address,
		client:    client.NewExecutorClient(cfg.Address),
	}
	return e, nil
}

func (m *mockExecutor) Execute(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error) {
	resp, err := m.client.ExecuteBatch(ctx, BatchToExecute(batch))
	if err != nil {
		return nil, err
	}
	result, err := m.ExecuteToBatch(batch.BatchID, resp)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (m *mockExecutor) ExecuteOne() string {
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

func (m *mockExecutor) ExecuteToBatch(batchId string, resp *v1.ExecuteBatchResponse) ([]*model.TaskResult, error) {
	var results []*model.TaskResult
	for _, item := range resp.GetResults() {
		var err error
		if item.ErrorMessage != "" {
			err = errors.New(errors.CodeInternal, item.ErrorMessage)
		}
		results = append(results, &model.TaskResult{
			TaskId:        item.TaskId,
			RequestId:     item.RequestId,
			ExecutorId:    m.id,
			Output:        item.OutputText,
			FinishReason:  item.FinishReason,
			ExecutionTime: time.Duration(item.ExecutionMs) * time.Millisecond,
			Usage: model.Usage{
				InputTokens:  item.InputTokens,
				OutputTokens: item.OutputTokens,
				TotalTokens:  item.InputTokens + item.OutputTokens,
			},
			Error:   err,
			BatchID: batchId,
			Timing: model.Timing{
				QueueMs:     0,
				BatchWaitMs: 0,
				ExecutionMs: item.ExecutionMs,
				TotalMs:     0,
			},
		})
	}

	return results, nil
}

func newHTTPExecutor(cfg conf.ExecutorConf) (Executor, error) {
	return nil, fmt.Errorf("http executor not implemented")
}
