package tools

import (
	"context"
	"encoding/json"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
)

type Env struct {
	CWD     string
	// Confirm func(ctx context.Context, prompt string) (bool, error)
}

type Tool interface {
	Defination() core.ToolDefination
	Execute(ctx context.Context, input json.RawMessage, env Env) (string, error)
}
