package gotopo

import (
	"errors"
	"fmt"
)

var (
	ErrNoMap          = errors.New("gotopo: no map is open")
	ErrNotFound       = errors.New("gotopo: feature not found")
	ErrAmbiguousMatch = errors.New("gotopo: multiple features matched")
	ErrClosed         = errors.New("gotopo: client is closed")
)

// APIError reports an HTTP or CalTopo response error.
type APIError struct {
	Method     string
	URL        string
	StatusCode int
	Status     string
	Message    string
	Body       string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("gotopo: %s %s: status %d: %s", e.Method, e.URL, e.StatusCode, e.Message)
	}
	return fmt.Sprintf("gotopo: %s %s: status %d", e.Method, e.URL, e.StatusCode)
}
