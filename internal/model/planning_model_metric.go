package model

import (
	"github.com/google/uuid"
	"time"
)

// PlanningModelMetric captures performance stats for a planning model scheme per published week.
type PlanningModelMetric struct {
	TenantScoped
	StoreID         uuid.UUID `gorm:"type:uuid;not null;index"`
	ModelScheme     string    `gorm:"not null;index"` // "A", "B", or custom name
	WeekStart       time.Time `gorm:"type:date;not null"`
	CoverageRate    float64   // 0–1.0 (fraction of days meeting minimum)
	OvertimeHours   float64   // total overtime hours generated this week
	AdjustmentCount int       // number of manual edits after model was applied
	ViolationCount  int       // hard violations at publish time
}
