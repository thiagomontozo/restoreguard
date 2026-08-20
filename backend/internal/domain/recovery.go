package domain

import (
	"errors"
	"time"
)

var ErrInvalidTransition = errors.New("invalid drill state transition")
var transitions = map[DrillStatus]map[DrillStatus]bool{
	DrillQueued: {DrillPreparing: true, DrillCancelled: true}, DrillPreparing: {DrillRestoring: true, DrillFailed: true, DrillCancelled: true, DrillInconclusive: true},
	DrillRestoring: {DrillValidating: true, DrillFailed: true, DrillCancelled: true, DrillInconclusive: true}, DrillValidating: {DrillFinalizing: true, DrillFailed: true, DrillCancelled: true, DrillInconclusive: true},
	DrillFinalizing: {DrillSucceeded: true, DrillFailed: true, DrillCancelled: true, DrillInconclusive: true}, DrillSucceeded: {}, DrillFailed: {}, DrillCancelled: {}, DrillInconclusive: {},
}

func CanTransition(from, to DrillStatus) bool { return transitions[from][to] }
func Transition(from, to DrillStatus) error {
	if !CanTransition(from, to) {
		return ErrInvalidTransition
	}
	return nil
}
func MeasureRPO(drillTime, snapshotTime time.Time) (int64, PolicyResult) {
	if snapshotTime.IsZero() || snapshotTime.After(drillTime) {
		return 0, PolicyInconclusive
	}
	return int64(drillTime.Sub(snapshotTime).Seconds()), ""
}
func MeasureRTO(startedAt, recoveryReadyAt time.Time) (int64, PolicyResult) {
	if startedAt.IsZero() || recoveryReadyAt.IsZero() || recoveryReadyAt.Before(startedAt) {
		return 0, PolicyInconclusive
	}
	return int64(recoveryReadyAt.Sub(startedAt).Seconds()), ""
}
func EvaluatePolicy(measured int64, measurement PolicyResult, target int64) PolicyResult {
	if measurement == PolicyInconclusive || measured < 0 || target <= 0 {
		return PolicyInconclusive
	}
	if measured <= target {
		return PolicyPass
	}
	return PolicyFail
}
func Assess(restorePassed bool, results []ValidationOutcome, evidenceIntegrity bool) (Assessment, Confidence) {
	if !restorePassed {
		return AssessmentFailed, ConfidenceLow
	}
	if len(results) == 0 {
		return AssessmentUnknown, ConfidenceLow
	}
	requiredFailed, limited := false, false
	for _, result := range results {
		if result.Required && result.Status == ValidationFail {
			requiredFailed = true
		}
		if result.Status == ValidationWarning || result.Status == ValidationInconclusive || (!result.Required && result.Status == ValidationFail) {
			limited = true
		}
	}
	if requiredFailed {
		return AssessmentFailed, ConfidenceLow
	}
	if limited || !evidenceIntegrity {
		return PartiallyVerified, ConfidenceMedium
	}
	return Verified, ConfidenceHigh
}
