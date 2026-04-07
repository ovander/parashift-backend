package repo

import (
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
	}
}
