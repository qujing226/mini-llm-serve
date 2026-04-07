package worker

import (
	"context"
	"sync/atomic"

	"github.com/qujing226/mini-llm-serve/internal/metrics"
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

	idx     atomic.Uint64
	metrics metrics.Metrics
}

func NewWorker(logger *zap.SugaredLogger, executors map[string]Executor, metrics metrics.Metrics) Worker {
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
		metrics:      metrics,
	}
	return e
}

func (e *work) Batch(ctx context.Context, batch *model.Batch) ([]*model.TaskResult, error) {
	executorId := e.executorList[e.idx.Load()%e.executorNum]
	executor := e.executors[executorId]
	e.idx.Add(1)
	// metrics: add batch process number for each executor
	e.metrics.IncBatches(executorId)

	resp, err := executor.Execute(ctx, batch)
	if err != nil {
		e.metrics.IncExecutorErrors(executorId)
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
