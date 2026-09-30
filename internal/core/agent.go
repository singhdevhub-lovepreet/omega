package core

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Budget struct {
	MaxTurns int
}

type EventType string

const (
	EventTextDelta  EventType = "text_delta"
	EventToolCall   EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventTurnEnd    EventType = "turn_end"
	EventRunEnd     EventType = "run_end"
	EventError      EventType = "error"
)

type RunEndReason string

const (
	RunEndComplete RunEndReason = "end_turn"
	RunEndBudget   RunEndReason = "budget"
	RunEndAborted  RunEndReason = "aborted"
)

type AgentEvent struct {
	Type EventType

	// text_delta
	Text string

	// tool_call / tool_result
	ToolUseID string
	ToolName  string
	Input     json.RawMessage
	Content   string
	IsError   bool

	// turn_end
	Usage      Usage
	StopReason StopReason

	// run_end
	FinalText  string
	TotalUsage Usage
	Reason     RunEndReason

	// error
	Err error
}

type Agent struct {
	provider Provider
	tools    ToolExecutor
	system   string
	budget   Budget
	messages []Message
	usage    Usage
}

func NewAgent(provider Provider, tools ToolExecutor, system string, budget Budget) *Agent {
	return &Agent{
		provider: provider,
		tools:    tools,
		system:   system,
		budget:   budget,
	}
}

func (a *Agent) Run(ctx context.Context, task string) <-chan AgentEvent {
	out := make(chan AgentEvent)

	go func() {
		defer close(out)
		defer func() {
			if r := recover(); r != nil {
				out <- AgentEvent{Type: EventError, Err: fmt.Errorf("panic: %v", r)}
			}
		}()

		a.messages = append(a.messages, Message{Role: RoleUser, Content: []ContentBlock{TextBlock(task)}})

		for turn := 0; turn < a.budget.MaxTurns; turn++ {
			res, err := a.provider.Complete(ctx, CompletionRequest{
				System:    a.system,
				Messages:  a.messages,
				Tools:     a.tools.Definitions(),
				MaxTokens: 4096,
			})
			if err != nil {
				if ctx.Err() != nil {
					out <- AgentEvent{Type: EventRunEnd, Reason: RunEndAborted, TotalUsage: a.usage}
					return
				}
				out <- AgentEvent{Type: EventError, Err: err}
				return
			}

			a.usage.InputTokens += res.Usage.InputTokens
			a.usage.OutputTokens += res.Usage.OutputTokens
			a.usage.CacheReadTokens += res.Usage.CacheReadTokens

			a.messages = append(a.messages, Message{Role: RoleAssistant, Content: res.Content})
			out <- AgentEvent{Type: EventTurnEnd, Usage: res.Usage, StopReason: res.StopReason}

			switch res.StopReason {
			case StopReasonEndTurn:
				out <- AgentEvent{Type: EventRunEnd, Reason: RunEndComplete, FinalText: textOf(res.Content), TotalUsage: a.usage}
				return
			case StopReasonMaxTokens:
				// tool_use blocks may be truncated mid-JSON; never execute them.
				out <- AgentEvent{Type: EventError, Err: fmt.Errorf("output truncated at max_tokens")}
				return
			}

			var results []ContentBlock
			for _, b := range res.Content {
				if b.Type != BlockToolUse {
					continue
				}
				out <- AgentEvent{Type: EventToolCall, ToolUseID: b.ID, ToolName: b.Name, Input: b.Input}
				content, isErr := a.tools.Execute(ctx, b.Name, b.Input)
				if ctx.Err() != nil {
					out <- AgentEvent{Type: EventRunEnd, Reason: RunEndAborted, TotalUsage: a.usage}
					return
				}
				out <- AgentEvent{Type: EventToolResult, ToolUseID: b.ID, Content: content, IsError: isErr}
				results = append(results, ToolResultBlock(b.ID, content, isErr))
			}
			a.messages = append(a.messages, Message{Role: RoleUser, Content: results})
		}

		out <- AgentEvent{Type: EventRunEnd, Reason: RunEndBudget, TotalUsage: a.usage}
	}()

	return out
}

func textOf(blocks []ContentBlock) string {
	var sb strings.Builder
	for _, b := range blocks {
		if b.Type == BlockText {
			sb.WriteString(b.Text)
		}
	}
	return sb.String()
}
