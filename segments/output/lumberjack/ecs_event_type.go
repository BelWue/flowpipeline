package lumberjack

import "time"

const (
	ECSEventDefaultModule   = "lumberjack"
	ECSEventDefaultProvider = "flowpipeline"
)

type ECSEventKind string

const (
	ECSEventKindUNSET         = ""
	ECSEventKindAlert         = "alert"
	ECSEventKindAsset         = "asset"
	ECSEventKindEnrichment    = "enrichment"
	ECSEventKindEvent         = "event"
	ECSEventKindMetric        = "metric"
	ECSEventKindState         = "state"
	ECSEventKindPipelineError = "pipeline_error"
	ECSEventKindSignal        = "signal"
)

type ECSEventCategory string

const (
	ECSEventCategoryUNSET              = ""
	ECSEventCategoryApi                = "api"
	ECSEventCategoryAuthentication     = "authentication"
	ECSEventCategoryConfiguration      = "configuration"
	ECSEventCategoryDatabase           = "database"
	ECSEventCategoryDriver             = "driver"
	ECSEventCategoryEmail              = "email"
	ECSEventCategoryFile               = "file"
	ECSEventCategoryHost               = "host"
	ECSEventCategoryIam                = "iam"
	ECSEventCategoryIntrusionDetection = "intrusion_detection"
	ECSEventCategoryLibrary            = "library"
	ECSEventCategoryMalware            = "malware"
	ECSEventCategoryNetwork            = "network"
	ECSEventCategoryPackage            = "package"
	ECSEventCategoryProcess            = "process"
	ECSEventCategoryRegistry           = "registry"
	ECSEventCategorySession            = "session"
	ECSEventCategoryThreat             = "threat"
	ECSEventCategoryVulnerability      = "vulnerability"
	ECSEventCategoryWeb                = "web"
)

type ECSEventType string

const (
	ECSEventTypeUNSET        = " "
	ECSEventTypeAccess       = "access"
	ECSEventTypeAdmin        = "admin"
	ECSEventTypeAllowed      = "allowed"
	ECSEventTypeChange       = "change"
	ECSEventTypeConnection   = "connection"
	ECSEventTypeCreation     = "creation"
	ECSEventTypeDeletion     = "deletion"
	ECSEventTypeDenied       = "denied"
	ECSEventTypeEnd          = "end"
	ECSEventTypeError        = "error"
	ECSEventTypeGroup        = "group"
	ECSEventTypeIndicator    = "indicator"
	ECSEventTypeInfo         = "info"
	ECSEventTypeInstallation = "installation"
	ECSEventTypeProtocol     = "protocol"
	ECSEventTypeStart        = "start"
	ECSEventTypeUser         = "user"
)

type ECSEventOutcome string

const (
	ECSEventOutcomeUNSET   ECSEventOutcome = ""        //
	ECSEventOutcomeFailure ECSEventOutcome = "failure" // failure
	ECSEventOutcomeSuccess ECSEventOutcome = "success" // success
	ECSEventOutcomeUnknown ECSEventOutcome = "unknown" // unknown
)

type ElasticTimeStamp time.Time

func (t ElasticTimeStamp) MarshalJSON() ([]byte, error) {
	return []byte(time.Time(t).Format(time.RFC3339)), nil
}

type ECSEvent struct {
	Kind     ECSEventKind       `json:"kind,omitempty"`
	Category []ECSEventCategory `json:"category,omitempty"`
	Type     []ECSEventType     `json:"type,omitempty"`
	Outcome  ECSEventOutcome    `json:"outcome,omitempty"`
	Created  int64              `json:"created,omitempty"`
	Start    uint64             `json:"start,omitempty"`
	End      uint64             `json:"end,omitempty"`
	Duration int64              `json:"duration,omitempty"`
	Provider string             `json:"provider,omitempty"`
	Module   string             `json:"module,omitempty"`
}
