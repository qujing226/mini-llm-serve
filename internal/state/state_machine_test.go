package state

import (
	"testing"
	"time"

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

func TestCanScheduleRejectsCanceledRequest(t *testing.T) {
	manager := NewRequestLifecycleStateManager(zap.NewNop().Sugar())
	req := &model.Request{
		RequestId:    "req-canceled",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
		PromptTokens: 2,
	}

	work, err := manager.Create(req)
	require.NoError(t, err)

	manager.Cancel(req.RequestId)
	require.False(t, manager.CanSchedule(work))
}

func TestCanScheduleRejectsTimedOutRequest(t *testing.T) {
	manager := NewRequestLifecycleStateManager(zap.NewNop().Sugar())
	req := &model.Request{
		RequestId:    "req-timeout",
		Model:        "mock",
		Prompt:       "hello",
		MaxTokens:    8,
		PromptTokens: 2,
		Deadline:     time.Now().Add(-time.Second),
	}

	work, err := manager.Create(req)
	require.NoError(t, err)
	ch, err := manager.Subscribe(req.RequestId)
	require.NoError(t, err)

	require.False(t, manager.CanSchedule(work))
	_, ok := manager.Get(req.RequestId)
	require.False(t, ok)

	event, ok := <-ch
	require.True(t, ok)
	require.Equal(t, v1.EventTypeRequestFailed, event.Type)
	require.True(t, event.Done)
	require.Error(t, event.Err)

	_, ok = <-ch
	require.False(t, ok)
}
