package client

import (
	"context"
	"net/http"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1/mini_llm_servev1connect"
)

type InferenceClient struct {
	httpClient      *http.Client
	endpoints       []string
	inferenceClient mini_llm_servev1connect.InferenceServiceClient
}

func NewClient(endpoints []string) *InferenceClient {
	return NewClientWithTimeout(endpoints, 5*time.Second)
}

func NewClientWithTimeout(endpoints []string, timeout time.Duration) *InferenceClient {
	transport := newLongConnTransport()
	c := &InferenceClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   timeout,
		},
		endpoints: endpoints,
	}
	c.dial()
	return c
}

func (c *InferenceClient) Generate(ctx context.Context, request *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	resp, err := c.inferenceClient.Generate(ctx, request)
	return resp, err
}

func (c *InferenceClient) dial() {
	c.inferenceClient = mini_llm_servev1connect.NewInferenceServiceClient(c.httpClient, c.endpoints[0])
}
