package client

import (
	"context"
	"net"
	"net/http"
	"time"

	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1/mini_llm_servev1connect"
)

type Client struct {
	httpClient      *http.Client
	endpoints       []string
	inferenceClient mini_llm_servev1connect.InferenceServiceClient
}

func NewClient(endpoints []string) *Client {
	transport := newLongConnTransport()
	c := &Client{
		httpClient: &http.Client{
			Transport: transport,
			Timeout:   time.Second * 5,
		},
		endpoints: endpoints,
	}
	c.dial()
	return c
}

func (c *Client) Generate(ctx context.Context, request *v1.GenerateRequest) (*v1.GenerateResponse, error) {
	resp, err := c.inferenceClient.Generate(ctx, request)
	return resp, err
}

func (c *Client) dial() {
	c.inferenceClient = mini_llm_servev1connect.NewInferenceServiceClient(c.httpClient, c.endpoints[0])
}

func newLongConnTransport() *http.Transport {
	return &http.Transport{
		// 长连接不需要连接池
		MaxIdleConns:        0,
		MaxIdleConnsPerHost: 0,
		MaxConnsPerHost:     0,

		// 不要回收空闲连接（保持长连接）
		IdleConnTimeout: 0,

		// TCP keepalive 非常重要
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second, // NAT/防火墙友好
		}).DialContext,

		TLSHandshakeTimeout: 10 * time.Second,
	}
}
