package model

// CoverageRequirement defines the minimum staffing requirement for a time slot.
type CoverageRequirement struct {
	TenantScoped
	DayOfWeek    int    `gorm:"not null"`   // 0-6
	StartTime    string `gorm:"not null"`   // HH:MM
	EndTime      string `gorm:"not null"`   // HH:MM
	MinStaff     int    `gorm:"not null;default:1"`
	RequiredRole string // e.g. "pharmacist", empty means any role acceptable
}
