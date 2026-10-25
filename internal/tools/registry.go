package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/singhdevhub-lovepreet/omega/internal/core"
)

type Registry struct {
	tools map[string]Tool // {"file_read" : Tool}, 1k tools
	env   Env
}

func NewRegistry(env Env, tools ...Tool) *Registry {
	r := &Registry{
		tools: make(map[string]Tool, len(tools)),
		env:   env,
	}
	for _, t := range tools {
		r.tools[t.Defination().Name] = t
	}
	return r
}

func (r *Registry) Definations() []core.ToolDefination {
	defs := make([]core.ToolDefination, 0, len(r.tools))
	for _, t := range r.tools {
		defs = append(defs, t.Defination())
	}
	return defs
}

func (r *Registry) Execute(ctx context.Context, name string, input json.RawMessage) (string, bool) {
	// "file_read", {...}. extract tool from map, execute
	t, ok := r.tools[name]
	if !ok {
		return fmt.Sprintf("Unknown tool %q", name), true
	}
	out, err := t.Execute(ctx, input, r.env)
	if err != nil {
		return err.Error(), true
	}
	return out, false
}

var _ core.ToolExecutor = (*Registry)(nil) // toolregistry
