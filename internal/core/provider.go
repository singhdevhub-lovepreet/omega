package core

import (
	"context"
	"encoding/json"
)

type Provider interface {
	Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error)
	// Stream(ctx context.Context, request CompletionRequest) (EventStream, error)
}

type ToolExecutor interface {
	Definations() []ToolDefination
	Execute(ctx context.Context, name string, input json.RawMessage) (content string, isError bool)
}
