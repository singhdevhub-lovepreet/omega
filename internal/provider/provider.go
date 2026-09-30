package provider

import (
	"context"
	"fmt"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
)

type Provider interface {
	Complete(ctx context.Context, request core.CompletionRequest) (core.CompletionResponse, error)
	// Stream(ctx context.Context, request core.CompletionRequest) (io.Reader, error)
}

type Error struct {
	Provider   string
	StatusCode int
	Type       string
	Message    string
	Retryable  bool
}

func (e *Error) Error() string {
	return fmt.Sprintf("%s: HTTP %d %s: %s", e.Provider, e.StatusCode, e.Type, e.Message)
}

func retryableStatus(code int) bool {
	return code == 408 || code == 429 || code >= 500
}
