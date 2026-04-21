package worker

import (
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
)

func BatchToExecute(batch *model.Batch) *v1.ExecuteBatchRequest {
	req := &v1.ExecuteBatchRequest{
		BatchId: batch.BatchID,
	}
	for _, r := range batch.Items {
		req.Items = append(req.Items, &v1.ExecuteItem{
			TaskId:    r.WorkId,
			RequestId: r.RequestId,
			Prompt:    r.Prompt,
			MaxTokens: r.MaxTokens,
		})
	}
	return req
}
