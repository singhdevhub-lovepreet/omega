package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
)

type ReadFileTool struct {
	maxSize    int64
	definition core.ToolDefinition
}

func (t *ReadFileTool) Definition() core.ToolDefinition {
	return t.definition
}

func (t *ReadFileTool) Execute(ctx context.Context, input json.RawMessage, env Env) (string, error) {
	var inputData struct {
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(input, &inputData); err != nil {
		return "", fmt.Errorf("invalid input: %w", err)
	}
	path, err := ResolvePath(env.CWD, inputData.FilePath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	content, err := io.ReadAll(io.LimitReader(f, t.maxSize+1))
	if err != nil {
		return "", err
	}
	if int64(len(content)) > t.maxSize {
		return "", fmt.Errorf("file exceeds %d bytes", t.maxSize)
	}
	return string(content), nil
}

func NewReadFileTool() Tool {
	return &ReadFileTool{
		maxSize: 1 << 20, // 1MB
		definition: core.ToolDefinition{
			Name:        "read_file",
			Description: "Read a file and return the content",
			InputSchema: json.RawMessage(
				`{
					"type": "object",
					"properties": {
						"file_path": {
							"type": "string",
							"description": "The path to the file to read"
						}
					},
					"required": ["file_path"],
					"additionalProperties": false
				}`,
			),
		},
	}
}
