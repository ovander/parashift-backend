package dto

import (
	"github.com/google/uuid"
)

// ── Coverage requirements (manager configuration) ─────────────────────────────

// CoverageRequirementRequest is the DTO for creating or updating a coverage
// requirement (the manager-defined staffing template per time slot).
type CoverageRequirementRequest struct {
	DayOfWeek    int     `json:"day_of_week" binding:"required,gte=0,lte=6"`
	StartTime    string  `json:"start_time"  binding:"required"` // HH:MM
	EndTime      string  `json:"end_time"    binding:"required"` // HH:MM
	MinStaff     int     `json:"min_staff"   binding:"required,gte=1"`
	RequiredRole *string `json:"required_role"`
}

// CoverageRequirementResponse is the canonical HTTP response for a coverage
// requirement entity.
type CoverageRequirementResponse struct {
	ID           uuid.UUID `json:"id"`
	StoreID      uuid.UUID `json:"store_id"`
	DayOfWeek    int       `json:"day_of_week"`
	StartTime    string    `json:"start_time"` // HH:MM
	EndTime      string    `json:"end_time"`   // HH:MM
	MinStaff     int       `json:"min_staff"`
	RequiredRole string    `json:"required_role"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
}

// ── Coverage analysis — internal aggregate (service layer) ───────────────────

// CoverageSlot represents the staffing status for a single time window on a
// specific date. Used internally by CoverageService and CoverageChecker.
//
// Date is included here so the service can produce a flat list; the HTTP
// handler groups slots by date into []CoverageDay for the API response.
//
// Status values: "OK" | "UNDERSTAFFED" | "OVERSTAFFED" | "MISSING_ROLE"
type CoverageSlot struct {
	Date          string `json:"date"`           // YYYY-MM-DD
	StartTime     string `json:"start_time"`     // HH:MM
	EndTime       string `json:"end_time"`       // HH:MM
	Status        string `json:"status"`
	AssignedCount int    `json:"assigned_count"`
	RequiredCount int    `json:"required_count"`
	MissingRole   bool   `json:"missing_role"`
	RequiredRole  string `json:"required_role"`  // the role string from the requirement, "" if none
}

// CoverageReport is the internal aggregate returned by CoverageService and the
// CoverageChecker interface. The HTTP handler converts this into []CoverageDay
// before writing the API response.
type CoverageReport struct {
	Items      []CoverageSlot `json:"items"`
	TotalSlots int            `json:"total_slots"`
	GapCount   int            `json:"gap_count"`
}

// ── Coverage analysis — canonical API response types ─────────────────────────

// CoverageSlotResponse is the public-facing slot shape returned by the API.
// It omits internal fields (Date, MissingRole) — the date lives on the
// parent CoverageDay and MissingRole is reflected in Status.
//
// Status values: "OK" | "UNDERSTAFFED" | "OVERSTAFFED"
type CoverageSlotResponse struct {
	StartTime     string `json:"start_time"`     // HH:MM
	EndTime       string `json:"end_time"`       // HH:MM
	RequiredCount int    `json:"required_count"`
	AssignedCount int    `json:"assigned_count"`
	Status        string `json:"status"` // OK|UNDERSTAFFED|OVERSTAFFED
}

// CoverageDay groups coverage slots for a single calendar date.
// This is the shape returned by GET /stores/{store_id}/coverage.
type CoverageDay struct {
	Date  string                 `json:"date"`  // YYYY-MM-DD
	Slots []CoverageSlotResponse `json:"slots"`
}

// GroupIntoCoverageDays converts a flat CoverageReport into the API response
// shape, grouping slots by date and mapping MISSING_ROLE → UNDERSTAFFED.
func GroupIntoCoverageDays(report CoverageReport) []CoverageDay {
	index := make(map[string]*CoverageDay)
	order := make([]string, 0, len(report.Items))

	for _, s := range report.Items {
		if _, exists := index[s.Date]; !exists {
			index[s.Date] = &CoverageDay{Date: s.Date, Slots: nil}
			order = append(order, s.Date)
		}

		// Map MISSING_ROLE → UNDERSTAFFED for external consumers.
		status := s.Status
		if status == "MISSING_ROLE" {
			status = "UNDERSTAFFED"
		}

		index[s.Date].Slots = append(index[s.Date].Slots, CoverageSlotResponse{
			StartTime:     s.StartTime,
			EndTime:       s.EndTime,
			RequiredCount: s.RequiredCount,
			AssignedCount: s.AssignedCount,
			Status:        status,
		})
	}

	days := make([]CoverageDay, 0, len(order))
	for _, date := range order {
		days = append(days, *index[date])
	}
	return days
}
