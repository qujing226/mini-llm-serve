package worker

import (
	"context"
	"sync/atomic"

	"github.com/qujing226/mini-llm-serve/internal/model"
	"go.uber.org/zap"
)

type Worker interface {
	Batch(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error)
	One(ctx context.Context, task *model.Task) (*model.TaskResult, error)
}

type work struct {
	logger       *zap.SugaredLogger
	executors    map[string]Executor
	executorList []string
	executorNum  uint64

	idx atomic.Uint64
}

func NewWorker(logger *zap.SugaredLogger, executors map[string]Executor) Worker {
	executorNum := len(executors)

	// todo: 调度管理
	executorList := make([]string, executorNum)
	idx := 0
	for s, _ := range executors {
		executorList[idx] = s
		idx++
	}

	e := &work{
		logger:       logger,
		executors:    executors,
		executorList: executorList,
		executorNum:  uint64(executorNum),
	}
	return e
}

func (e *work) Batch(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error) {
	executor := e.executors[e.executorList[e.idx.Load()%e.executorNum]]
	e.idx.Add(1)
	resp, err := executor.Execute(ctx, batch)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (e *work) One(ctx context.Context, task *model.Task) (*model.TaskResult, error) {
	return &model.TaskResult{
		TaskId:        task.TaskId,
		RequestId:     task.RequestId,
		ExecutorId:    "",
		Output:        "",
		FinishReason:  0,
		ExecutionTime: 0,
		Usage:         model.Usage{},
		Error:         nil,
		BatchID:       "",
		Timing:        model.Timing{},
	}, nil
	//var ec *http.Client
	//if time.Now().Sub(task.DeadLine) > time.Second {
	//	ec = e.executors["deepseek"]
	//} else {
	//	ec = e.executors["openai"]
	//}

	//res, err := ec.Post("http://127.0.0.1:9991/deepseek", "application/json", io.Writer(task.Prompt))
	//if err != nil {
	//	return nil, err
	//}

	//return res
}
