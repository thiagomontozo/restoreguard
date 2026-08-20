package domain

import (
	"errors"
	"testing"
	"time"
)

func TestDrillTransitions(t *testing.T) {
	if err := Transition(DrillQueued, DrillPreparing); err != nil {
		t.Fatal(err)
	}
	if err := Transition(DrillQueued, DrillSucceeded); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected invalid transition, got %v", err)
	}
}
func TestRPOAndPolicy(t *testing.T) {
	now := time.Date(2026, 8, 19, 20, 0, 0, 0, time.UTC)
	seconds, state := MeasureRPO(now, now.Add(-4*time.Hour))
	if seconds != 14400 || EvaluatePolicy(seconds, state, 86400) != PolicyPass {
		t.Fatalf("unexpected RPO: %d %s", seconds, state)
	}
	if EvaluatePolicy(seconds, state, 3600) != PolicyFail {
		t.Fatal("expected RPO fail")
	}
}
func TestRTOWithFakeClockValues(t *testing.T) {
	start := time.Date(2026, 8, 19, 20, 0, 0, 0, time.UTC)
	seconds, state := MeasureRTO(start, start.Add(31*time.Minute))
	if EvaluatePolicy(seconds, state, 1800) != PolicyFail {
		t.Fatal("expected RTO fail")
	}
}
func TestAssessmentSemantics(t *testing.T) {
	assessment, confidence := Assess(true, []ValidationOutcome{{Status: ValidationPass, Required: true}}, true)
	if assessment != Verified || confidence != ConfidenceHigh {
		t.Fatalf("unexpected %s/%s", assessment, confidence)
	}
	assessment, _ = Assess(true, []ValidationOutcome{{Status: ValidationFail, Required: true}}, true)
	if assessment != AssessmentFailed {
		t.Fatalf("required validation failure must fail, got %s", assessment)
	}
}
