package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
)

// EmployeeLookup is consumed by ScheduleService and CoverageService.
type EmployeeLookup interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Employee, error)
	ListByStore(ctx context.Context, tenantID uuid.UUID) ([]*model.Employee, error)
}

// ShiftLookup is consumed by CoverageService and SwapService.
type ShiftLookup interface {
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error)
	ListByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error)
}

// CoverageChecker is consumed by ScheduleService to validate coverage after assignment changes.
type CoverageChecker interface {
	ComputeForDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (dto.CoverageReport, error)
}
