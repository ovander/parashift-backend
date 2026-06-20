package repo

import (
	"context"

	"gorm.io/gorm"
)

// RepoBundle holds all repository instances and the shared database connection.
type RepoBundle struct {
	DB                      *gorm.DB
	Store                   StoreRepository
	Employee                EmployeeRepository
	Contract                ContractRepository
	WeekTemplate            WeekTemplateRepository
	CoverageRequirement     CoverageRequirementRepository
	ShiftInstance           ShiftInstanceRepository
	ShiftAssignment         ShiftAssignmentRepository
	Availability            AvailabilityRepository
	LeaveRequest            LeaveRequestRepository
	ShiftSlot               ShiftSlotRepository
	SwapRequest             SwapRequestRepository
	AuditLog                AuditLogRepository
	Rule                    RuleRepository
	AIInsight               AIInsightRepository
	SchedulePlan            SchedulePlanRepository
	Qualification           QualificationRepository
	EmployeeQualification   EmployeeQualificationRepository
	PlanningModelMetric     PlanningModelMetricRepository
	PublicHoliday           PublicHolidayRepository
	StoreException          StoreExceptionRepository
	TokenRevocation         TokenRevocationRepository
}

// WithTx runs fn inside a single database transaction (DAT-1). fn receives a
// RepoBundle whose repositories all operate on the transaction, so multi-repo
// operations commit or roll back atomically. Returning a non-nil error — or
// panicking — rolls the transaction back; returning nil commits it.
//
// Example:
//
//	err := repos.WithTx(ctx, func(tx *repo.RepoBundle) error {
//	    if err := tx.ShiftInstance.CreateBatch(ctx, shifts); err != nil { return err }
//	    return tx.ShiftAssignment.CreateBatch(ctx, assignments)
//	})
func (b *RepoBundle) WithTx(ctx context.Context, fn func(tx *RepoBundle) error) error {
	return b.DB.WithContext(ctx).Transaction(func(txDB *gorm.DB) error {
		return fn(NewRepoBundle(txDB))
	})
}

// AdvisoryXactLock takes a Postgres transaction-level advisory lock identified by
// key. It must be called inside a transaction (e.g. within WithTx); the lock is
// released automatically when that transaction commits or rolls back. Concurrent
// callers with the same key block until the holder's transaction ends, which
// serializes otherwise-racy multi-step operations (DAT-3 idempotent generate).
// A no-op when DB is nil (e.g. fake bundles in unit tests).
func (b *RepoBundle) AdvisoryXactLock(ctx context.Context, key int64) error {
	if b.DB == nil {
		return nil
	}
	return b.DB.WithContext(ctx).Exec("SELECT pg_advisory_xact_lock(?)", key).Error
}

// NewRepoBundle creates a new repository bundle with all concrete implementations.
func NewRepoBundle(db *gorm.DB) *RepoBundle {
	return &RepoBundle{
		DB:                      db,
		Store:                   NewStoreRepository(db),
		Employee:                NewEmployeeRepository(db),
		Contract:                NewContractRepository(db),
		WeekTemplate:            NewWeekTemplateRepository(db),
		CoverageRequirement:     NewCoverageRequirementRepository(db),
		ShiftInstance:           NewShiftInstanceRepository(db),
		ShiftAssignment:         NewShiftAssignmentRepository(db),
		Availability:            NewAvailabilityRepository(db),
		LeaveRequest:            NewLeaveRequestRepository(db),
		ShiftSlot:               NewShiftSlotRepository(db),
		SwapRequest:             NewSwapRequestRepository(db),
		AuditLog:                NewAuditLogRepository(db),
		Rule:                    NewRuleRepository(db),
		AIInsight:               NewAIInsightRepository(db),
		SchedulePlan:            NewSchedulePlanRepository(db),
		Qualification:           NewQualificationRepository(db),
		EmployeeQualification:   NewEmployeeQualificationRepository(db),
		PlanningModelMetric:     NewPlanningModelMetricRepository(db),
		PublicHoliday:           NewPublicHolidayRepository(db),
		StoreException:          NewStoreExceptionRepository(db),
		TokenRevocation:         NewTokenRevocationRepository(db),
	}
}
