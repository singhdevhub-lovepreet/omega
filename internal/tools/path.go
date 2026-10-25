package tools

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ResolvePath(root, p string) (string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("resolve workspace root: %w", err)
	}
	abs := filepath.Clean(filepath.Join(root, p))
	if abs != root && !strings.HasPrefix(abs, root+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q escapes workspace", p)
	}
	return abs, nil
}
