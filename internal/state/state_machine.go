package state

import (
	"sync"
	"sync/atomic"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/internal/errors"
	"github.com/qujing226/mini-llm-serve/internal/metrics"
	"github.com/qujing226/mini-llm-serve/internal/model"
	"github.com/qujing226/mini-llm-serve/internal/utils"
	"go.uber.org/zap"
)

type RequestLifecycleStateManager interface {
	Create(req *model.Request) (*model.WorkItem, error)
	Get(requestId string) (*model.Request, bool)
	Subscribe(requestId string) (<-chan *model.Event, error)

	CanSchedule(work *model.WorkItem) bool
	OnEvent(e *model.Event) ([]*model.WorkItem, error)
	Fail(requestId string, err error)

	Cancel(requestId string)
	Finish(requestId string)
}

type requestLifecycleStateManager struct {
	l *zap.SugaredLogger

	requests    map[string]*model.Request
	subscribeCh map[string]chan *model.Event
	mu          sync.RWMutex

	activeRequests atomic.Int64
	metrics        metrics.Metrics
}

func NewRequestLifecycleStateManager(l *zap.SugaredLogger, metrics metrics.Metrics) RequestLifecycleStateManager {
	r := &requestLifecycleStateManager{
		l:           l,
		requests:    make(map[string]*model.Request),
		subscribeCh: make(map[string]chan *model.Event),
		metrics:     metrics,
	}
	return r
}

func (r *requestLifecycleStateManager) Create(req *model.Request) (*model.WorkItem, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.requests[req.RequestId]; exists {
		return nil, errors.New(errors.CodeInvalidArgument, "request already exists")
	}
	r.requests[req.RequestId] = req
	req.Phase = model.RequestPhasePrefillReady

	r.subscribeCh[req.RequestId] = make(chan *model.Event, 5)

	// metrics: add active requests
	r.increaseActiveRequest()

	now := time.Now()
	workItem := &model.WorkItem{
		WorkId:        utils.MustGenerateUUIDv7(),
		RequestId:     req.RequestId,
		Phase:         v1.WorkPhasePrefill,
		Model:         req.Model,
		Prompt:        req.Prompt,
		MaxTokens:     req.MaxTokens,
		Deadline:      req.Deadline,
		PromptTokens:  req.PromptTokens,
		PrefillOffset: 0,
		PrefillTokens: req.PromptTokens,
		CacheHit:      false,
		ReadyAt:       now,
	}
	return workItem, nil
}

func (r *requestLifecycleStateManager) Get(requestId string) (*model.Request, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if req, exists := r.requests[requestId]; exists {
		return req, true
	}
	return nil, false
}
func (r *requestLifecycleStateManager) Subscribe(requestId string) (<-chan *model.Event, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ch, exists := r.subscribeCh[requestId]
	if !exists {
		return nil, errors.New(errors.CodeInternal, "subscribe channel doesn't exist")
	}
	return ch, nil
}

func (r *requestLifecycleStateManager) CanSchedule(work *model.WorkItem) bool {
	r.mu.Lock()
	req, exists := r.requests[work.RequestId]
	if !exists {
		r.mu.Unlock()
		return false
	}
	if !req.Deadline.IsZero() && time.Now().After(req.Deadline) {
		req.Phase = model.RequestPhaseTimeout
		delete(r.requests, work.RequestId)
		r.reduceActiveRequest()
		subCh, exists := r.subscribeCh[work.RequestId]
		delete(r.subscribeCh, work.RequestId)
		r.mu.Unlock()
		if exists {
			subCh <- &model.Event{
				WorkId:       utils.MustGenerateUUIDv7(),
				RequestId:    work.RequestId,
				Type:         v1.EventTypeRequestFailed,
				Done:         true,
				FinishReason: v1.FinishReasonError,
				At:           time.Now(),
				Err:          errors.New(errors.CodeRequestTimeout, "request timeout"),
			}
			close(subCh)
		}
		return false
	}

	switch req.Phase {
	case model.RequestPhaseFinished, model.RequestPhaseCanceled, model.RequestPhaseTimeout, model.RequestPhaseFailed:
		r.mu.Unlock()
		return false
	default:
		r.mu.Unlock()
		return true
	}
}

func (r *requestLifecycleStateManager) OnEvent(e *model.Event) ([]*model.WorkItem, error) {
	r.mu.Lock()
	req, exists := r.requests[e.RequestId]
	if !exists {
		r.mu.Unlock()
		// The request may have been canceled or finished while a work item was
		// still queued or in-flight. Treat late executor results as stale.
		return nil, nil
	}

	var onWorkItems []*model.WorkItem
	now := time.Now()
	switch e.Type {
	case v1.EventTypePrefillChunk:
		req.Phase = model.RequestPhasePrefillRunning
		prefillItem := &model.WorkItem{
			WorkId:        utils.MustGenerateUUIDv7(),
			RequestId:     e.RequestId,
			Phase:         v1.WorkPhasePrefill,
			Model:         req.Model,
			Prompt:        req.Prompt,
			MaxTokens:     req.MaxTokens,
			Deadline:      req.Deadline,
			PromptTokens:  req.PromptTokens,
			PrefillOffset: e.Usage.InputTokens,
			PrefillTokens: req.PromptTokens - e.Usage.InputTokens,
			CacheHit:      false,
			ReadyAt:       now,
		}
		onWorkItems = append(onWorkItems, prefillItem)
	case v1.EventTypePrefillFinished:
		req.Phase = model.RequestPhaseDecodeReady
		decodeItem := &model.WorkItem{
			WorkId:       utils.MustGenerateUUIDv7(),
			RequestId:    e.RequestId,
			Phase:        v1.WorkPhaseDecode,
			Model:        req.Model,
			Prompt:       req.Prompt,
			MaxTokens:    req.MaxTokens,
			Deadline:     req.Deadline,
			PromptTokens: req.PromptTokens,
			CacheHit:     false,
			ReadyAt:      now,
		}
		onWorkItems = append(onWorkItems, decodeItem)
	case v1.EventTypeDecodeChunk:
		req.Usage = e.Usage
		req.OutputText += e.DeltaText
		// todo: GeneratedTokens = e.Usage.OutputTokens 假设 executor 返回的是累计值。这个后面要和 Python/mock 或真实 backend 的 usage 语义对齐。现在先接受。
		req.GeneratedTokens = e.Usage.OutputTokens
		if e.Done {
			req.Phase = model.RequestPhaseFinished
			req.FinishedAt = now
			req.FinishReason = e.FinishReason
			// todo：决定清理请求/通知订阅者
		} else {
			req.Phase = model.RequestPhaseDecodeRunning
			decodeItem := &model.WorkItem{
				WorkId:       utils.MustGenerateUUIDv7(),
				RequestId:    e.RequestId,
				Phase:        v1.WorkPhaseDecode,
				Model:        req.Model,
				Prompt:       req.Prompt,
				MaxTokens:    req.MaxTokens,
				Deadline:     req.Deadline,
				PromptTokens: req.PromptTokens,
				CacheHit:     false,
				ReadyAt:      now,
			}
			onWorkItems = append(onWorkItems, decodeItem)
		}

	case v1.EventTypeRequestFinished:
		req.Phase = model.RequestPhaseFinished
	case v1.EventTypeRequestFailed:
		req.Phase = model.RequestPhaseFailed
	case v1.EventTypeRequestCanceled:
		req.Phase = model.RequestPhaseCanceled
	}

	r.mu.Unlock()

	// publish event
	r.mu.RLock()
	subCh, exists := r.subscribeCh[e.RequestId]
	r.mu.RUnlock()
	if exists {
		// todo: subCh <- e 仍然可能阻塞。后面如果流式消费者慢，状态机会被拖住。现在先不动。
		subCh <- e
	} else {
		r.l.Errorf("request %s not subscribed", e.RequestId)
	}

	return onWorkItems, nil
}

func (r *requestLifecycleStateManager) Cancel(requestId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseCanceled
		delete(r.requests, requestId)
		r.reduceActiveRequest()
	}
	if subCh, exists := r.subscribeCh[requestId]; exists {
		delete(r.subscribeCh, requestId)
		close(subCh)
	}

}

func (r *requestLifecycleStateManager) Fail(requestId string, err error) {
	r.mu.Lock()
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseFailed
		delete(r.requests, requestId)
		r.reduceActiveRequest()
	}
	subCh, exists := r.subscribeCh[requestId]
	delete(r.subscribeCh, requestId)
	r.mu.Unlock()
	if exists {
		subCh <- &model.Event{
			WorkId:       utils.MustGenerateUUIDv7(),
			RequestId:    requestId,
			Type:         v1.EventTypeRequestFailed,
			Done:         true,
			FinishReason: v1.FinishReasonError,
			At:           time.Now(),
			Err:          err,
		}
		close(subCh)
	}
}

func (r *requestLifecycleStateManager) Finish(requestId string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if req, exists := r.requests[requestId]; exists {
		req.Phase = model.RequestPhaseFinished
		delete(r.requests, requestId)
		r.reduceActiveRequest()
	}
	if subCh, exists := r.subscribeCh[requestId]; exists {
		delete(r.subscribeCh, requestId)
		close(subCh)
	}
}

func (r *requestLifecycleStateManager) increaseActiveRequest() {
	r.metrics.SetActiveRequests(int(r.activeRequests.Add(1)))

}
func (r *requestLifecycleStateManager) reduceActiveRequest() {
	r.metrics.SetActiveRequests(int(r.activeRequests.Add(-1)))
}
