package state

import (
	"testing"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestOnEventIgnoresStaleEventAfterCancel(t *testing.T) {
	manager := NewRequestLifecycleStateManager(zap.NewNop().Sugar())
	req := &model.Request{
		RequestId:    "req-stale",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
		PromptTokens: 2,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)
	require.NotNil(t, work)

	manager.Cancel(req.RequestId)

	next, err := manager.OnEvent(&model.Event{
		WorkId:    work.WorkId,
		RequestId: req.RequestId,
		Type:      v1.EventTypeDecodeChunk,
		Done:      false,
	})

	require.NoError(t, err)
	require.Nil(t, next)
}
