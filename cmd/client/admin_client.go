package client

import (
	"context"
	"net/http"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1/mini_llm_servev1connect"
)

type AdminClient struct {
	httpClient *http.Client
	endpoints  []string
	client     mini_llm_servev1connect.AdminServiceClient
}

func NewAdminClient(endpoints []string) *AdminClient {
	transport := newLongConnTransport()
	a := &AdminClient{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   10 * time.Second,
		},
		endpoints: endpoints,
	}
	a.dial()
	return a
}

func (a *AdminClient) dial() {
	a.client = mini_llm_servev1connect.NewAdminServiceClient(a.httpClient, a.endpoints[0])
}

func (a *AdminClient) Health(ctx context.Context, request *v1.HealthRequest) (*v1.HealthResponse, error) {
	return a.client.Health(ctx, request)
}

func (a *AdminClient) GetRuntimeStats(ctx context.Context, request *v1.GetRuntimeStatsRequest) (*v1.GetRuntimeStatsResponse, error) {
	return a.client.GetRuntimeStats(ctx, request)
}
