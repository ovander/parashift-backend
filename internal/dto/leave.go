package dto

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateLeaveRequest is the DTO for creating a leave request.
// StartDate / EndDate are plain YYYY-MM-DD strings so they survive JSON decoding
// without requiring RFC3339 from the caller. The handler parses them to time.Time.
type CreateLeaveRequest struct {
	StartDate string  `json:"start_date"` // YYYY-MM-DD
	EndDate   string  `json:"end_date"`   // YYYY-MM-DD
	Type      string  `json:"type"`       // vacation|sick|other
	Reason    *string `json:"reason"`
}

// ParsedDates parses StartDate / EndDate into time.Time. Returns an error if
// either field is empty or not in YYYY-MM-DD format.
func (r CreateLeaveRequest) ParsedDates() (start, end time.Time, err error) {
	if r.StartDate == "" || r.EndDate == "" {
		return start, end, fmt.Errorf("start_date and end_date are required (YYYY-MM-DD)")
	}
	start, err = time.Parse("2006-01-02", r.StartDate)
	if err != nil {
		return start, end, fmt.Errorf("invalid start_date, use YYYY-MM-DD")
	}
	end, err = time.Parse("2006-01-02", r.EndDate)
	if err != nil {
		return start, end, fmt.Errorf("invalid end_date, use YYYY-MM-DD")
	}
	return
}

// ReviewLeaveRequest is the DTO for reviewing a leave request.
type ReviewLeaveRequest struct {
	Status string `json:"status" binding:"required"` // approved|rejected
}

// LeaveImpactShift describes one shift that would be affected by approving a leave.
type LeaveImpactShift struct {
	Date          string `json:"date"`           // YYYY-MM-DD
	StartTime     string `json:"start_time"`     // HH:MM
	EndTime       string `json:"end_time"`       // HH:MM
	Role          string `json:"role"`
	// OtherAssigned is the number of OTHER non-cancelled assignments on this shift.
	// 0 means this employee is the sole cover — approving will leave the shift empty.
	OtherAssigned int    `json:"other_assigned"`
	WillNeedCover bool   `json:"will_need_cover"` // true when OtherAssigned == 0
}

// LeaveImpactResponse is returned by GET /leave-requests/:id/impact.
// It lets the manager preview what will be cancelled before approving.
type LeaveImpactResponse struct {
	// Shifts the employee is assigned to during the leave period.
	AffectedShifts []LeaveImpactShift `json:"affected_shifts"`
	// Total number of assignments that will be cancelled.
	TotalCancellations int `json:"total_cancellations"`
	// Total scheduled hours that will be lost.
	TotalHoursLost float64 `json:"total_hours_lost"`
	// Number of shifts that will be left with no cover at all.
	UncoveredShifts int `json:"uncovered_shifts"`
}

// LeaveRequestResponse is the DTO for returning leave request information.
type LeaveRequestResponse struct {
	ID        uuid.UUID `json:"id"`
	TenantID  uuid.UUID `json:"tenant_id"`
	EmployeeID uuid.UUID `json:"employee_id"`
	StartDate string    `json:"start_date"`
	EndDate   string    `json:"end_date"`
	Type      string    `json:"type"`
	Status    string    `json:"status"`
	Reason    string    `json:"reason"`
	ReviewedBy *uuid.UUID `json:"reviewed_by"`
	ReviewedAt *string   `json:"reviewed_at"`
	CreatedAt string    `json:"created_at"`
	UpdatedAt string    `json:"updated_at"`
}
