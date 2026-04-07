package model

import "github.com/google/uuid"

const (
	// AIInsight types
	InsightTypeAnomaly        = "ANOMALY"         // unusual pattern detected
	InsightTypeUnderstaffed   = "UNDERSTAFFED"    // coverage gap identified
	InsightTypeOverstaffed    = "OVERSTAFFED"     // excess staffing detected
	InsightTypeImbalance      = "IMBALANCE"       // workload not evenly distributed
	InsightTypeOptimization   = "OPTIMIZATION"    // schedule improvement suggestion
)

// AIInsight is a generated AI observation about a store's schedule or staffing.
// Unlike audit logs, insights are advisory and carry a confidence score.
type AIInsight struct {
	TenantScoped
	Type           string     `gorm:"not null"`
	Message        string     `gorm:"not null"`
	Recommendation string
	Confidence     float64    `gorm:"not null;default:0"`
	RelatedShiftID *uuid.UUID `gorm:"type:uuid"` // optional — shift this insight refers to
	RelatedEmpID   *uuid.UUID `gorm:"type:uuid"` // optional — employee this insight refers to
	Dismissed      bool       `gorm:"not null;default:false"`
}
