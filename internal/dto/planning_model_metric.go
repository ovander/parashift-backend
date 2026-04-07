package dto

type PlanningModelMetricResponse struct {
	Scheme          string  `json:"scheme"`
	WeekStart       string  `json:"week_start"`
	CoverageRate    float64 `json:"coverage_rate"`
	OvertimeHours   float64 `json:"overtime_hours"`
	AdjustmentCount int     `json:"adjustment_count"`
	ViolationCount  int     `json:"violation_count"`
}

type PlanningModelMetricSummary struct {
	AvgCoverageRate    float64 `json:"avg_coverage_rate"`
	AvgOvertimeHours   float64 `json:"avg_overtime_hours"`
	AvgAdjustmentCount float64 `json:"avg_adjustment_count"`
	SampleWeeks        int     `json:"sample_weeks"`
}

type PlanningModelMetricsResponse struct {
	Scheme  string                            `json:"scheme"`
	Metrics []PlanningModelMetricResponse      `json:"metrics"`
	Summary PlanningModelMetricSummary        `json:"summary"`
}
