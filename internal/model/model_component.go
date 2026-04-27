package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

type Batch struct {
	BatchID   string
	BatchSize uint64
	Phase     v1.WorkPhase
	CreateAt  time.Time
	Items     []*WorkItem
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
