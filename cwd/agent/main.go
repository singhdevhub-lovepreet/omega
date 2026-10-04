package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
	"github.com/singhdevhub-lovepreet/omega/internal/provider"
	"github.com/singhdevhub-lovepreet/omega/internal/tools"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: omega <task>")
		os.Exit(2)
	}
	task := os.Args[1]

	key := os.Getenv("OPENROUTER_API_KEY")
	model := os.Getenv("OPENROUTER_MODEL")
	if key == "" || model == "" {
		fmt.Fprintln(os.Stderr, "error: OPENROUTER_API_KEY or OPENROUTER_MODEL is not set")
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: get cwd: %v\n", err)
		os.Exit(1)
	}

	system := "You are a helpful coding agent. When a user refers to files or directories, always use the available tools to inspect them before answering. Base your answer only on tool outputs, not prior knowledge."
	registry := tools.NewRegistry(tools.Env{CWD: cwd}, tools.NewReadFileTool())
	agent := core.NewAgent(
		provider.NewOpenRouterProvider(model, key),
		registry,
		system,
		core.Budget{MaxTurns: 10},
	)

	exitCode := 0
	for event := range agent.Run(ctx, task) {
		switch event.Type {
		case core.EventError:
			fmt.Fprintf(os.Stderr, "error: %v\n", event.Err)
			exitCode = 1
		case core.EventRunEnd:
			fmt.Fprint(os.Stdout, event.FinalText)
			fmt.Fprintf(os.Stderr, "\nrun ended: %s\n", event.Reason)
		case core.EventTurnEnd:
			fmt.Fprintf(os.Stderr, "· turn · in=%d out=%d · %s\n", event.Usage.InputTokens, event.Usage.OutputTokens, event.StopReason)
		case core.EventToolCall:
			fmt.Fprintf(os.Stderr, "→ %s %s\n", event.ToolName, event.Input)
		case core.EventToolResult:
			fmt.Fprintf(os.Stderr, "← %s (isError=%v)\n", event.Content, event.IsError)
		case core.EventTextDelta:
			fmt.Fprint(os.Stdout, event.Text)
		default:
			fmt.Fprintf(os.Stderr, "unknown event %q\n", event.Type)
		}
	}

	os.Exit(exitCode)
}
