package analyze

import "time"

const SchemaVersion = "1.1"

type Result struct {
	SchemaVersion string      `json:"schemaVersion"`
	Project       string      `json:"project"`
	Service       string      `json:"service"`
	Window        Window      `json:"window"`
	Status        string      `json:"status"`
	SLOs          []SLOResult `json:"slos"`
	Errors        []string    `json:"errors"`
}

type Window struct {
	Start           time.Time `json:"start"`
	End             time.Time `json:"end"`
	DurationSeconds int64     `json:"durationSeconds"`
}

type SLOResult struct {
	SLOResourceName         string   `json:"sloResourceName"`
	SLOID                   string   `json:"sloId"`
	DisplayName             string   `json:"displayName"`
	Goal                    float64  `json:"goal"`
	RollingPeriodDays       int64    `json:"rollingPeriodDays"`
	CalendarPeriod          *string  `json:"calendarPeriod"`
	Compliance              float64  `json:"compliance"`
	BadFraction             float64  `json:"badFraction"`
	AllowedBadFraction      float64  `json:"allowedBadFraction"`
	ConsumedPercentOfBudget float64  `json:"consumedPercentOfBudget"`
	Status                  string   `json:"status"`
	Explain                 *Explain `json:"explain,omitempty"`
	Error                   string   `json:"error,omitempty"`
}

type Explain struct {
	Formula string   `json:"formula"`
	Notes   []string `json:"notes"`
}

type Sources struct {
	Project     string   `json:"project"`
	Service     string   `json:"service"`
	ServiceName string   `json:"serviceName"`
	Start       string   `json:"start"`
	End         string   `json:"end"`
	SLOs        []string `json:"slos"`
}
