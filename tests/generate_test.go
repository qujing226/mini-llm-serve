package tests

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGenerate(t *testing.T) {
	requireServer(t, "127.0.0.1:8800")
	c := client.NewClient([]string{"http://127.0.0.1:8800"})
	var wg sync.WaitGroup

	msgNumber := 100
	errCh := make(chan error, msgNumber)

	for i := 0; i < msgNumber; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
				RequestId: "001" + strconv.Itoa(i),
				Model:     "deepseek-v4",
				Prompt:    "hello world",
				MaxTokens: 8,
				TimeoutMs: 1500,
				Labels:    nil,
			})
			if err != nil {
				errCh <- err
				return
			}
			if resp == nil {
				errCh <- fmt.Errorf("nil response")
				return
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
	resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
		RequestId: "002",
		Model:     "deepseek-v4",
		Prompt:    "hello world",
		MaxTokens: 8,
		TimeoutMs: 10000,
		Labels:    nil,
	})
	require.NoError(t, err)
	r, err := protojson.MarshalOptions{
		Indent:          "  ",
		EmitUnpopulated: true,
	}.Marshal(resp)
	require.NoError(t, err)
	t.Log(string(r))
}

func requireServer(t *testing.T, addr string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		t.Skipf("skip integration test: server %s unavailable: %v", addr, err)
	}
	_ = conn.Close()
}
