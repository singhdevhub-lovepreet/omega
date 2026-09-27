package core

type Budget struct {
	MaxTurns int
}

type EventType string

const (
	EventTextDelta EventType = "text_delta"
	EventToolCall EventType = "tool_call"
	EventToolResult EventType = "tool_result"
	EventTurnEnd EventType = "turn_end"
	EventRunEnd EventType = "run_end"
	EventError EventType = "error"
)

type RunEndReason string

const (
	RunEndComplete RunEndReason = "end_turn"
	RunEndBudget RunEndReason = "budget"
	RunEndAborted RunEndReason = "aborted"
)

type AgentEvent struct {
	Type EventType
	
	// text_delta
	Text string
	
	// tool_call / tool_result
	ToolUseID string
	ToolName string
	Input json.RawMessage
	Content string
	IsError bool

	// turn_end
	Usage Usage
	StopReason StopReason

	// run_end
	FinalText string
	TotalUsage Usage
	Reason RunEndReason

	// error
	Err error

}

type Agent struct {
	provider Provider
	tools ToolExecutor
	system string
	budget Budget
	messages []Message
	usage Usage
}

func NewAgent(provider Provider, tools ToolExecutor, system string, budget Budget) *Agent

func (a *Agent) Run(ctx context.Context, task string) <- chan AgentEvent