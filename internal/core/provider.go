package core

import (
	"context"
	"encoding/json"
)

type Provider interface {
	Complete(ctx context.Context, request CompletionRequest) (CompletionResponse, error)
}

type ToolExecutor interface {
	Definitions() []ToolDefinition
	Execute(ctx context.Context, name string, input json.RawMessage) (content string, isError bool)
}
