package worker

import (
	"net/http"

	"github.com/qujing226/mini-llm-serve/internal/model"
)

type Executor interface {
	Execute(batch *model.Batch) ([]*model.TaskResult, error)
	ExecuteOne(task *model.Task) (*model.TaskResult, error)
}

type executor struct {
	executors map[string]*http.Client
}

func NewExecutor(executors map[string]*http.Client) Executor {
	e := &executor{
		executors: executors,
	}
	return e
}

func (e *executor) Execute(batch *model.Batch) ([]*model.TaskResult, error) {
	panic("implement me")
}

func (e *executor) ExecuteOne(task *model.Task) (*model.TaskResult, error) {
	return &model.TaskResult{
		TaskId:        "",
		RequestId:     "",
		WorkerId:      "",
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
