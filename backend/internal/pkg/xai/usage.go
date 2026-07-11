package xai

type UsageSnapshot struct {
	Source       string   `json:"source,omitempty"`
	ValueName    string   `json:"value_name,omitempty"`
	USD          float64  `json:"usd"`
	UsageName    string   `json:"usage_name,omitempty"`
	Usage        *float64 `json:"usage,omitempty"`
	StartTime    string   `json:"start_time,omitempty"`
	EndTime      string   `json:"end_time,omitempty"`
	Timezone     string   `json:"timezone,omitempty"`
	LimitReached bool     `json:"limit_reached,omitempty"`
	UpdatedAt    string   `json:"updated_at"`
}
