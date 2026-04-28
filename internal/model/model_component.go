package model

import (
	"time"
)

type Batch struct {
	BatchID   string
	BatchSize uint64
	CreateAt  time.Time
	Items     []*WorkItem
}

type Usage struct {
	InputTokens  uint64
	OutputTokens uint64
	TotalTokens  uint64
}

type Timing struct {
	Queue     time.Duration
	BatchWait time.Duration
	Execution time.Duration
	Total     time.Duration
}
