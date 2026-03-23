package pipe

import (
	"testing"
)

func TestTotalCallsEmpty(t *testing.T) {
	p := &Plan{}
	if got := p.TotalCalls(); got != 0 {
		t.Errorf("TotalCalls() = %d, want 0", got)
	}
}

func TestTotalCallsNoSteps(t *testing.T) {
	p := &Plan{Steps: []StepPlan{}}
	if got := p.TotalCalls(); got != 0 {
		t.Errorf("TotalCalls() = %d, want 0", got)
	}
}

func TestTotalCallsSingleStep(t *testing.T) {
	p := &Plan{
		Steps: []StepPlan{
			{Calls: []Call{{Job: "a"}, {Job: "b"}, {Job: "c"}}},
		},
	}
	if got := p.TotalCalls(); got != 3 {
		t.Errorf("TotalCalls() = %d, want 3", got)
	}
}

func TestTotalCallsMultipleSteps(t *testing.T) {
	p := &Plan{
		Steps: []StepPlan{
			{Calls: []Call{{Job: "a"}, {Job: "b"}}},
			{Calls: []Call{{Job: "c"}}},
			{Calls: []Call{{Job: "d"}, {Job: "e"}, {Job: "f"}, {Job: "g"}}},
		},
	}
	if got := p.TotalCalls(); got != 7 {
		t.Errorf("TotalCalls() = %d, want 7", got)
	}
}

func TestTotalCallsStepWithNoCalls(t *testing.T) {
	p := &Plan{
		Steps: []StepPlan{
			{Calls: []Call{{Job: "a"}}},
			{Calls: nil},
			{Calls: []Call{{Job: "b"}}},
		},
	}
	if got := p.TotalCalls(); got != 2 {
		t.Errorf("TotalCalls() = %d, want 2", got)
	}
}
