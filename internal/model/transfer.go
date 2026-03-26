package model

import (
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
)

func ProtoMsgToModel(in *v1.GenerateRequest) (*GenerateInput, error) {
	out := &GenerateInput{
		RequestId: in.RequestId,
		Model:     in.Model,
		Prompt:    in.Prompt,
		MaxTokens: in.MaxTokens,
		Timeout:   time.Duration(int64(in.TimeoutMs)) * time.Millisecond,
		Labels:    in.Labels,
	}
	return out, nil
}

func ModelToProtoMsg(in *GenerateOutput) (*v1.GenerateResponse, error) {
	usage := &v1.Usage{
		InputTokens:  in.Usage.InputTokens,
		OutputTokens: in.Usage.OutputTokens,
		TotalTokens:  in.Usage.TotalTokens,
	}

	timing := &v1.Timing{
		QueueMs:     in.Timing.QueueMs,
		BatchWaitMs: in.Timing.BatchWaitMs,
		ExecutionMs: in.Timing.ExecutionMs,
		TotalMs:     in.Timing.TotalMs,
	}

	batch := &v1.BatchInfo{
		BatchId:   in.BatchID,
		BatchSize: in.BatchSize,
	}

	out := &v1.GenerateResponse{
		RequestId:    in.RequestId,
		OutputText:   in.Output,
		FinishReason: in.FinishReason,
		Usage:        usage,
		Timing:       timing,
		Batch:        batch,
		ExecutorId:   in.ExecutorId,
	}

	return out, nil
}
