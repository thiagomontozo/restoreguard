package domain

import "time"

type DrillStatus string

const (
	DrillQueued       DrillStatus = "QUEUED"
	DrillPreparing    DrillStatus = "PREPARING"
	DrillRestoring    DrillStatus = "RESTORING"
	DrillValidating   DrillStatus = "VALIDATING"
	DrillFinalizing   DrillStatus = "FINALIZING"
	DrillSucceeded    DrillStatus = "SUCCEEDED"
	DrillFailed       DrillStatus = "FAILED"
	DrillCancelled    DrillStatus = "CANCELLED"
	DrillInconclusive DrillStatus = "INCONCLUSIVE"
)

type Assessment string

const (
	Verified          Assessment = "VERIFIED"
	PartiallyVerified Assessment = "PARTIALLY_VERIFIED"
	AssessmentFailed  Assessment = "FAILED"
	AssessmentUnknown Assessment = "INCONCLUSIVE"
)

type PolicyResult string

const (
	PolicyPass         PolicyResult = "PASS"
	PolicyFail         PolicyResult = "FAIL"
	PolicyInconclusive PolicyResult = "INCONCLUSIVE"
)

type Confidence string

const (
	ConfidenceHigh   Confidence = "HIGH"
	ConfidenceMedium Confidence = "MEDIUM"
	ConfidenceLow    Confidence = "LOW"
)

type ValidationStatus string

const (
	ValidationPass         ValidationStatus = "PASS"
	ValidationFail         ValidationStatus = "FAIL"
	ValidationWarning      ValidationStatus = "WARNING"
	ValidationInconclusive ValidationStatus = "INCONCLUSIVE"
)

type RecoveryDrill struct {
	ID                 string       `json:"id"`
	OrganizationID     string       `json:"organizationId"`
	ProtectedAssetID   string       `json:"protectedAssetId"`
	BackupSnapshotID   string       `json:"backupSnapshotId"`
	RecoveryPolicyID   string       `json:"recoveryPolicyId"`
	RequestedBy        string       `json:"requestedBy"`
	TriggerType        string       `json:"triggerType"`
	Status             DrillStatus  `json:"status"`
	StartedAt          *time.Time   `json:"startedAt,omitempty"`
	CompletedAt        *time.Time   `json:"completedAt,omitempty"`
	MeasuredRPOSeconds *int64       `json:"measuredRpoSeconds,omitempty"`
	MeasuredRTOSeconds *int64       `json:"measuredRtoSeconds,omitempty"`
	RPOResult          PolicyResult `json:"rpoResult,omitempty"`
	RTOResult          PolicyResult `json:"rtoResult,omitempty"`
	RecoveryStatus     Assessment   `json:"recoveryStatus,omitempty"`
	Confidence         Confidence   `json:"confidence,omitempty"`
	Summary            string       `json:"summary"`
	CreatedAt          time.Time    `json:"createdAt"`
	UpdatedAt          time.Time    `json:"updatedAt"`
}
type ValidationOutcome struct {
	Status   ValidationStatus
	Required bool
}
type RecoveryPolicy struct {
	RPOTargetSeconds int64
	RTOTargetSeconds int64
}
