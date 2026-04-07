package tests

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGenerate(t *testing.T) {
	c := client.NewClient([]string{"http://127.0.0.1:8800"})
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
				RequestId: "001" + strconv.Itoa(i),
				Model:     "",
				Prompt:    "",
				MaxTokens: 0,
				TimeoutMs: 3000,
				Labels:    nil,
			})
			require.NoError(t, err)
			require.NotNil(t, resp)
			require.Equal(t, resp.FinishReason, v1.FinishReasonStop)
			wg.Done()
		}(i)
	}
	wg.Wait()
	resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
		RequestId: "002",
		Model:     "",
		Prompt:    "",
		MaxTokens: 0,
		TimeoutMs: 3000,
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
