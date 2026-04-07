package dto

import (
	"github.com/google/uuid"
)

type CreateSchedulePlanRequest struct {
	WeekStart string `json:"week_start"` // "YYYY-MM-DD", always a Monday
	Scheme    string `json:"scheme,omitempty"`
}

type PublishPlanRequest struct {
	Note string `json:"note,omitempty"` // optional manager note for this publish
}

type RollbackPlanRequest struct {
	SnapshotVersion int    `json:"snapshot_version"`
	Note            string `json:"note,omitempty"`
}

type RecordOverrideRequest struct {
	ShiftID uuid.UUID `json:"shift_id"`
	Reason  string    `json:"reason,omitempty"`
}

type SchedulePlanResponse struct {
	ID          uuid.UUID  `json:"id"`
	StoreID     uuid.UUID  `json:"store_id"`
	WeekStart   string     `json:"week_start"`
	State       string     `json:"state"`
	PublishedAt *string    `json:"published_at,omitempty"`
	PublishedBy *uuid.UUID `json:"published_by,omitempty"`
	Version     int        `json:"version"` // number of snapshots
	CreatedAt   string     `json:"created_at"`
	UpdatedAt   string     `json:"updated_at"`
}

type PlanSnapshotResponse struct {
	Version     int       `json:"version"`
	CapturedAt  string    `json:"captured_at"`
	ShiftCount  int       `json:"shift_count"`
	PublishedBy uuid.UUID `json:"published_by"`
}
