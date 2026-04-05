package pipe

import (
	"errors"
	"strings"
	"testing"
)

func TestRunErrorWithErr(t *testing.T) {
	e := &RunError{
		Job:      "scripts/deploy.sh",
		ExitCode: 1,
		Err:      errors.New("permission denied"),
	}
	got := e.Error()
	if got != "scripts/deploy.sh: exit 1: permission denied" {
		t.Errorf("Error() = %q, want %q", got, "scripts/deploy.sh: exit 1: permission denied")
	}
}

func TestRunErrorWithoutErr(t *testing.T) {
	e := &RunError{
		Job:      "scripts/build.py",
		ExitCode: 2,
		Err:      nil,
	}
	got := e.Error()
	if got != "scripts/build.py: exit 2" {
		t.Errorf("Error() = %q, want %q", got, "scripts/build.py: exit 2")
	}
}

func TestRunErrorZeroExitCode(t *testing.T) {
	e := &RunError{
		Job:      "scripts/check.sh",
		ExitCode: 0,
		Err:      errors.New("signal: killed"),
	}
	got := e.Error()
	if got != "scripts/check.sh: exit 0: signal: killed" {
		t.Errorf("Error() = %q, want %q", got, "scripts/check.sh: exit 0: signal: killed")
	}
}

func TestValidationErrorSingle(t *testing.T) {
	e := &ValidationError{
		Errors: []error{errors.New("step 0: missing job")},
	}
	got := e.Error()
	if !strings.HasPrefix(got, "1 validation error:") {
		t.Errorf("Error() = %q, want prefix %q", got, "1 validation error:")
	}
	if !strings.Contains(got, "  - step 0: missing job") {
		t.Errorf("Error() = %q, want to contain %q", got, "  - step 0: missing job")
	}
}

func TestValidationErrorMultiple(t *testing.T) {
	e := &ValidationError{
		Errors: []error{
			errors.New("step 0: missing job"),
			errors.New("step 1: unknown input mode"),
			errors.New("loop: invalid date range"),
		},
	}
	got := e.Error()
	if !strings.HasPrefix(got, "3 validation errors:") {
		t.Errorf("Error() = %q, want prefix %q", got, "3 validation errors:")
	}
	for _, msg := range []string{
		"  - step 0: missing job",
		"  - step 1: unknown input mode",
		"  - loop: invalid date range",
	} {
		if !strings.Contains(got, msg) {
			t.Errorf("Error() = %q, missing %q", got, msg)
		}
	}
}

func TestRunErrorImplementsErrorInterface(t *testing.T) {
	var err error = &RunError{Job: "test", ExitCode: 1}
	if err.Error() == "" {
		t.Error("expected non-empty error string")
	}
}

func TestValidationErrorImplementsErrorInterface(t *testing.T) {
	var err error = &ValidationError{Errors: []error{errors.New("x")}}
	if err.Error() == "" {
		t.Error("expected non-empty error string")
	}
}
