package tests

import (
	"context"
	"testing"

	"github.com/qujing226/mini-llm-serve/cmd/client"
	v1 "github.com/qujing226/mini-llm-serve/gen/go/mini_llm_serve/v1"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

func TestGenerate(t *testing.T) {
	c := client.NewClient([]string{"http://127.0.0.1:8800"})
	resp, err := c.Generate(context.Background(), &v1.GenerateRequest{
		RequestId: "001",
		Model:     "",
		Prompt:    "",
		MaxTokens: 0,
		TimeoutMs: 0,
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
