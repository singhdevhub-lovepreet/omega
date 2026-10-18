package provider

import "fmt"

type Error struct {
	Provider   string
	StatusCode int
	Type       string
	Message    string
	Retryable  bool
}

func (e *Error) Error() string {
	return fmt.Sprint("%s: HTTP %d %s: %s", e.Provider, e.StatusCode, e.Type, e.Message)
}

func retryableStatus(code int) bool {
	return code == 408 || code == 429 || code >= 500
}
