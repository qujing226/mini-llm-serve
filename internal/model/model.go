package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

type Request struct {
	RequestId string
	Model     string
	Prompt    string
	MaxTokens uint32
	Timeout   time.Duration
	Deadline  time.Time

	CacheKey string

	PromptTokens    uint32
	GeneratedTokens uint32
	ChunkTokens     uint32

	Phase        RequestPhase
	FinishReason v1.FinishReason

	OutputText   string
	CreatedAt    time.Time
	FirstTokenAt time.Time
	FinishedAt   time.Time

	Usage Usage

	Labels map[string]string
}

// GenerateOutput stage1
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

type WorkItem struct {
	WorkId    string
	RequestId string
	Phase     v1.WorkPhase

	Model     string
	Prompt    string
	MaxTokens uint32
	Deadline  time.Time

	PromptTokens        uint32
	DecodeTokensPlanned uint32
	BudgetCost          uint32

	CacheHit bool

	EnqueuedAt time.Time
	ReadyAt    time.Time
}

type Event struct {
	WorkId     string
	RequestId  string
	BatchId    string
	ExecutorId string

	Type v1.EventType

	ChunkIndex uint32
	DeltaText  string
	Done       bool

	Usage        Usage
	Timing       Timing
	FinishReason v1.FinishReason

	At  time.Time
	Err error
}

type Batch struct {
	BatchID   string
	BatchSize uint64
	CreateAt  time.Time
	Items     []*WorkItem
}

type RuntimeStats struct {
	QueueLength      uint64
	InflightRequests uint64
	InflightBatches  uint64
	BusyExecutors    uint64
	IdleExecutors    uint64
}

type Usage struct {
	InputTokens  uint32
	OutputTokens uint32
	TotalTokens  uint32
}

type Timing struct {
	Queue     time.Duration
	BatchWait time.Duration
	Execution time.Duration
	Total     time.Duration
}
