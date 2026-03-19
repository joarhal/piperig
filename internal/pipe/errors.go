package pipe

import (
	"fmt"
	"strings"
)

// ValidationError collects multiple validation errors.
type ValidationError struct {
	Errors []error
}

func (e *ValidationError) Error() string {
	msgs := make([]string, len(e.Errors))
	for i, err := range e.Errors {
		msgs[i] = "  - " + err.Error()
	}
	return fmt.Sprintf("%d validation errors:\n%s", len(e.Errors), strings.Join(msgs, "\n"))
}

// RunError represents a job execution failure.
type RunError struct {
	Job      string
	ExitCode int
	Err      error
}

func (e *RunError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: exit %d: %v", e.Job, e.ExitCode, e.Err)
	}
	return fmt.Sprintf("%s: exit %d", e.Job, e.ExitCode)
}
