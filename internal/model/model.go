package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

type GenerateInput struct {
	RequestId string
	Model     string
	Prompt    string
	MaxTokens uint32
	Timeout   time.Duration
	Labels    map[string]string
}

type GenerateOutput struct {
	RequestId    string
	Output       string
	FinishReason v1.FinishReason
	Usage        Usage
	Timing       Timing
	BatchID      string
	BatchSize    uint32
	ExecutorId   string
}

type Task struct {
	TaskId     string
	RequestId  string
	Model      string
	Prompt     string
	MaxTokens  uint32
	Labels     map[string]string
	EnqueuedAt time.Time
	DeadLine   time.Time
	ResultCh   chan *TaskResult
}

type TaskResult struct {
	TaskId     string
	RequestId  string
	ExecutorId string

	Output string

	FinishReason  v1.FinishReason
	ExecutionTime time.Duration
	Usage         Usage

	Error   error
	BatchID string
	Timing  Timing
}

type Batch struct {
	BatchID   string
	BatchSize int64
	CreateAt  time.Time
	Tasks     []*Task
}

type RuntimeStats struct{}

type Usage struct {
	InputTokens  uint32
	OutputTokens uint32
	TotalTokens  uint32
}

type Timing struct {
	QueueMs     uint32
	BatchWaitMs uint32
	ExecutionMs uint32
	TotalMs     uint32
}
