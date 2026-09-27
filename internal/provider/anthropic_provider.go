package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
)

const (
	anthropicMessagesURL = "https://api.anthropic.com/v1/messages"
	anthropicVersion     = "2023-06-01"
)

type AnthropicProvider struct {
	model      string
	apiKey     string
	httpClient *http.Client
}

func NewAnthropicProvider(model, apiKey string) *AnthropicProvider {
	return &AnthropicProvider{
		model:  model,
		apiKey: apiKey,
		// Timeout bounds a hung connection; ctx handles user/budget cancel.
		// Generous because a long tool-heavy turn can legitimately take a while.
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
}

type anthropicMessage struct {
	Role    string           `json:"role"`
	Content []anthropicBlock `json:"content"`
}

type anthropicBlock struct {
	Type      string          `json:"type"`
	Text      string          `json:"text,omitempty"`
	ID        string          `json:"id,omitempty"`
	Name      string          `json:"name,omitempty"`
	Input     json.RawMessage `json:"input,omitempty"`
	ToolUseID string          `json:"tool_use_id,omitempty"`
	Content   string          `json:"content,omitempty"`
	IsError   bool            `json:"is_error,omitempty"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type anthropicUsage struct {
	InputTokens              int `json:"input_tokens"`
	OutputTokens             int `json:"output_tokens"`
	CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
}

type anthropicResponse struct {
	ID         string           `json:"id"`
	Role       string           `json:"role"`
	Content    []anthropicBlock `json:"content"`
	StopReason string           `json:"stop_reason"`
	Usage      anthropicUsage   `json:"usage"`
}

type anthropicError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
		Code    string `json:"code"`
	} `json:"error"`
}

func (ap *AnthropicProvider) toWire(request core.CompletionRequest) (anthropicRequest, error) {
	messages := make([]anthropicMessage, len(request.Messages))
	for i, message := range request.Messages {
		messages[i] = anthropicMessage{
			Role:    string(message.Role),
			Content: blocksToWire(message.Content),
		}
	}

	tools := make([]anthropicTool, len(request.Tools))
	for i, tool := range request.Tools {
		tools[i] = anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		}
	}

	return anthropicRequest{
		Model:     ap.model,
		MaxTokens: request.MaxTokens,
		System:    request.System,
		Messages:  messages,
		Tools:     tools,
	}, nil
}

func blocksToWire(blocks []core.ContentBlock) []anthropicBlock {
	out := make([]anthropicBlock, len(blocks))
	for i, b := range blocks {
		out[i] = anthropicBlock{
			Type:      string(b.Type),
			Text:      b.Text,
			ID:        b.ID,
			Name:      b.Name,
			Input:     b.Input,
			ToolUseID: b.ToolUseID,
			Content:   b.Content,
			IsError:   b.IsError,
		}
	}
	return out
}

func (ap *AnthropicProvider) fromWire(response anthropicResponse) (core.CompletionResponse, error) {
	stopReason, err := stopReasonFromWire(response.StopReason)
	if err != nil {
		return core.CompletionResponse{}, err
	}

	return core.CompletionResponse{
		Content:    blocksFromWire(response.Content),
		StopReason: stopReason,
		Usage: core.Usage{
			InputTokens:     response.Usage.InputTokens,
			OutputTokens:    response.Usage.OutputTokens,
			CacheReadTokens: response.Usage.CacheReadInputTokens,
		},
	}, nil
}

func blocksFromWire(blocks []anthropicBlock) []core.ContentBlock {
	out := make([]core.ContentBlock, len(blocks))
	for i, b := range blocks {
		out[i] = core.ContentBlock{
			Type:      core.BlockType(b.Type),
			Text:      b.Text,
			ID:        b.ID,
			Name:      b.Name,
			Input:     b.Input,
			ToolUseID: b.ToolUseID,
			Content:   b.Content,
			IsError:   b.IsError,
		}
	}
	return out
}

func stopReasonFromWire(s string) (core.StopReason, error) {
	switch s {
	case "end_turn", "stop_sequence":
		return core.StopReasonEndTurn, nil
	case "tool_use":
		return core.StopReasonToolUse, nil
	case "max_tokens":
		return core.StopReasonMaxTokens, nil
	default:
		return "", fmt.Errorf("anthropic: unknown stop_reason %q", s)
	}
}

func (ap *AnthropicProvider) Complete(ctx context.Context, request core.CompletionRequest) (core.CompletionResponse, error) {
	wireReq, err := ap.toWire(request)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("anthropic: to wire: %w", err)
	}
	body, err := json.Marshal(wireReq)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("anthropic: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicMessagesURL, bytes.NewReader(body))
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("anthropic: build request: %w", err)
	}
	req.Header.Set("x-api-key", ap.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)
	req.Header.Set("content-type", "application/json")

	resp, err := ap.httpClient.Do(req)
	if err != nil {
		// ctx cancellation surfaces here wrapped; the loop checks ctx.Err() first,
		// so we return as-is rather than wrapping it in a provider.Error.
		return core.CompletionResponse{}, fmt.Errorf("anthropic: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return core.CompletionResponse{}, ap.errorFromResponse(resp)
	}

	var wireResp anthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return core.CompletionResponse{}, fmt.Errorf("anthropic: decode: %w", err)
	}

	return ap.fromWire(wireResp)
}

func (ap *AnthropicProvider) errorFromResponse(resp *http.Response) error {
	perr := &Error{
		Provider:   "anthropic",
		StatusCode: resp.StatusCode,
		Type:       http.StatusText(resp.StatusCode),
		Retryable:  retryableStatus(resp.StatusCode),
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var wireErr anthropicError
	if json.Unmarshal(raw, &wireErr) == nil && wireErr.Error.Message != "" {
		perr.Type = wireErr.Error.Type
		perr.Message = wireErr.Error.Message
	} else {
		perr.Message = string(bytes.TrimSpace(raw))
	}
	return perr
}
