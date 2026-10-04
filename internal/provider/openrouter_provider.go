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
	openrouterChatURL = "https://openrouter.ai/api/v1/chat/completions"
	openrouterTitle   = "omega"
)

type OpenRouterProvider struct {
	model      string
	apiKey     string
	httpClient *http.Client
}

func NewOpenRouterProvider(model, apiKey string) *OpenRouterProvider {
	return &OpenRouterProvider{
		model:  model,
		apiKey: apiKey,
		// OpenRouter can proxy to slow upstreams; give it room without being unbounded.
		httpClient: &http.Client{Timeout: 5 * time.Minute},
	}
}

// ---- wire types: private to this file, mirror OpenAI / OpenRouter chat-completions exactly ----

type openrouterRequest struct {
	Model     string              `json:"model"`
	Messages  []openrouterMessage `json:"messages"`
	Tools     []openrouterTool    `json:"tools,omitempty"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
}

type openrouterMessage struct {
	Role       string               `json:"role"`
	Content    string               `json:"content,omitempty"`
	ToolCalls  []openrouterToolCall `json:"tool_calls,omitempty"`
	ToolCallID string               `json:"tool_call_id,omitempty"`
	Name       string               `json:"name,omitempty"`
}

type openrouterTool struct {
	Type     string                `json:"type"`
	Function openrouterFunctionDef `json:"function"`
}

type openrouterFunctionDef struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type openrouterToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Function openrouterFunctionCall `json:"function"`
}

type openrouterFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openrouterResponse struct {
	ID      string             `json:"id"`
	Model   string             `json:"model"`
	Choices []openrouterChoice `json:"choices"`
	Usage   openrouterUsage    `json:"usage"`
}

type openrouterChoice struct {
	Index        int               `json:"index"`
	Message      openrouterMessage `json:"message"`
	FinishReason string            `json:"finish_reason"`
	Usage        openrouterUsage   `json:"usage,omitempty"`
}

type openrouterUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type openrouterError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

var _ core.Provider = (*OpenRouterProvider)(nil)

// Complete is the only side effect: translate → marshal → POST → status check → decode → translate back.
func (op *OpenRouterProvider) Complete(ctx context.Context, request core.CompletionRequest) (core.CompletionResponse, error) {
	wireReq, err := op.toWire(request)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("openrouter: to wire: %w", err)
	}
	body, err := json.Marshal(wireReq)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("openrouter: marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openrouterChatURL, bytes.NewReader(body))
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("openrouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+op.apiKey)
	req.Header.Set("Content-Type", "application/json")
	// Optional but recommended for OpenRouter rankings/identification.
	req.Header.Set("X-OpenRouter-Title", openrouterTitle)

	resp, err := op.httpClient.Do(req)
	if err != nil {
		return core.CompletionResponse{}, fmt.Errorf("openrouter: send: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return core.CompletionResponse{}, op.errorFromResponse(resp)
	}

	var wireResp openrouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&wireResp); err != nil {
		return core.CompletionResponse{}, fmt.Errorf("openrouter: decode: %w", err)
	}
	if len(wireResp.Choices) == 0 {
		return core.CompletionResponse{}, fmt.Errorf("openrouter: empty choices")
	}

	return op.fromWire(wireResp.Choices[0])
}

// ---- neutral → wire ----

func (op *OpenRouterProvider) toWire(request core.CompletionRequest) (openrouterRequest, error) {
	messages := make([]openrouterMessage, 0, 1+len(request.Messages))
	messages = append(messages, openrouterMessage{Role: "system", Content: request.System})

	for _, message := range request.Messages {
		msgs, err := messagesToOpenRouter(message)
		if err != nil {
			return openrouterRequest{}, err
		}
		messages = append(messages, msgs...)
	}

	tools := make([]openrouterTool, len(request.Tools))
	for i, tool := range request.Tools {
		tools[i] = openrouterTool{
			Type: "function",
			Function: openrouterFunctionDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		}
	}

	return openrouterRequest{
		Model:     op.model,
		Messages:  messages,
		Tools:     tools,
		MaxTokens: request.MaxTokens,
	}, nil
}

// messagesToOpenRouter converts one neutral Message into one or more wire messages.
// A user message containing tool results becomes N {role:"tool"} messages; everything
// else maps to a single message with content + tool_calls inline.
func messagesToOpenRouter(message core.Message) ([]openrouterMessage, error) {
	switch message.Role {
	case core.RoleUser:
		// tool_result blocks become separate {role:"tool"} messages; remaining text is ignored.
		var results []openrouterMessage
		for _, b := range message.Content {
			if b.Type == core.BlockToolResult {
				results = append(results, openrouterMessage{
					Role:       "tool",
					ToolCallID: b.ToolUseID,
					Content:    b.Content,
				})
			}
		}
		// If there were no tool_result blocks, it's a regular user message.
		if len(results) == 0 {
			return []openrouterMessage{{Role: "user", Content: textOf(message.Content)}}, nil
		}
		return results, nil

	case core.RoleAssistant:
		msg := openrouterMessage{Role: "assistant"}
		msg.Content = textOf(message.Content)
		for _, b := range message.Content {
			if b.Type == core.BlockToolUse {
				// OpenAI/OpenRouter expect arguments as a string, not an object.
				args, err := json.Marshal(b.Input)
				if err != nil {
					return nil, fmt.Errorf("marshal arguments for tool %q: %w", b.Name, err)
				}
				msg.ToolCalls = append(msg.ToolCalls, openrouterToolCall{
					ID:   b.ID,
					Type: "function",
					Function: openrouterFunctionCall{
						Name:      b.Name,
						Arguments: string(args),
					},
				})
			}
		}
		return []openrouterMessage{msg}, nil

	default:
		return nil, fmt.Errorf("openrouter: unknown role %q", message.Role)
	}
}

// ---- wire → neutral ----

func (op *OpenRouterProvider) fromWire(choice openrouterChoice) (core.CompletionResponse, error) {
	stopReason, err := stopReasonFromOpenRouter(choice.FinishReason)
	if err != nil {
		return core.CompletionResponse{}, err
	}

	var blocks []core.ContentBlock
	if choice.Message.Content != "" {
		blocks = append(blocks, core.TextBlock(choice.Message.Content))
	}
	for _, tc := range choice.Message.ToolCalls {
		// arguments come as a JSON string; parse back into a raw object for core.
		var input json.RawMessage
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &input); err != nil {
			return core.CompletionResponse{}, fmt.Errorf("parse arguments for tool %q: %w", tc.Function.Name, err)
		}
		blocks = append(blocks, core.ToolUseBlock(tc.ID, tc.Function.Name, input))
	}

	return core.CompletionResponse{
		Content:    blocks,
		StopReason: stopReason,
		Usage: core.Usage{
			InputTokens:     choice.Usage.PromptTokens,
			OutputTokens:    choice.Usage.CompletionTokens,
			CacheReadTokens: 0,
		},
	}, nil
}

func stopReasonFromOpenRouter(s string) (core.StopReason, error) {
	switch s {
	case "stop":
		return core.StopReasonEndTurn, nil
	case "tool_calls":
		return core.StopReasonToolUse, nil
	case "length":
		return core.StopReasonMaxTokens, nil
	default:
		return "", fmt.Errorf("openrouter: unknown finish_reason %q", s)
	}
}

// errorFromResponse mirrors anthropic's handler; both use shared *provider.Error.
func (op *OpenRouterProvider) errorFromResponse(resp *http.Response) error {
	perr := &Error{
		Provider:   "openrouter",
		StatusCode: resp.StatusCode,
		Type:       http.StatusText(resp.StatusCode),
		Retryable:  retryableStatus(resp.StatusCode),
	}

	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	var wireErr openrouterError
	if json.Unmarshal(raw, &wireErr) == nil && wireErr.Error.Message != "" {
		perr.Type = wireErr.Error.Type
		perr.Message = wireErr.Error.Message
	} else {
		perr.Message = string(bytes.TrimSpace(raw))
	}
	return perr
}

// textOf joins text blocks; used where the wire format needs a single string.
func textOf(blocks []core.ContentBlock) string {
	var sb bytes.Buffer
	for _, b := range blocks {
		if b.Type == core.BlockText {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}
