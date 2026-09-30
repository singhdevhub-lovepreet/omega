package agent

import (
	"context"
	"fmt"
	"os"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
	"github.com/singhdevhub-lovepreet/omega/internal/provider"
	"github.com/singhdevhub-lovepreet/omega/internal/tools"
)

func main() {
	allArgs := os.Args
	if len(allArgs) < 2 {
		fmt.Println("Usage: omega <task>")
	}
	task := allArgs[1]
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cwd, _ := os.Getwd()
	registry := tools.NewRegistry(tools.Env{CWD: cwd}, tools.NewReadFileTool())
	agent := core.NewAgent(provider.NewAnthropicProvider("claude-3-5-sonnet-20260912", os.Getenv("ANTHROPIC_API_KEY")), registry, "", core.Budget{MaxTurns: 10})
	agent.Run(ctx, task)
}
