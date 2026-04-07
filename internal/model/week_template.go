package model

import (
	"github.com/google/uuid"
)

// WeekTemplate represents a scheduled shift template for an employee on a specific day of the A/B week cycle.
// Multiple rows per (employee, week_type, day_of_week) are allowed to support split shifts (e.g. 9-12 and 13-19).
type WeekTemplate struct {
	TenantScoped
	EmployeeID uuid.UUID `gorm:"type:uuid;not null;index:idx_week_template_emp_type_day"`
	WeekType   string    `gorm:"not null;index:idx_week_template_emp_type_day"` // A|B
	DayOfWeek  int       `gorm:"not null;index:idx_week_template_emp_type_day"` // 0=Sunday, 1=Monday...6=Saturday
	StartTime  string    `gorm:"not null"`                                      // HH:MM
	EndTime    string    `gorm:"not null"`                                      // HH:MM
	Role       string    `gorm:"not null;default:''"`                           // job role required for this slot (e.g. pharmacist_assistant)
}
