package repo

import (
	"context"
	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"time"
)

// StoreRepository defines operations for store management.
type StoreRepository interface {
	// GetByID retrieves a store by its ID.
	GetByID(ctx context.Context, id uuid.UUID) (*model.Store, error)
	// List retrieves all stores with pagination.
	List(ctx context.Context, page, pageSize int) ([]*model.Store, int64, error)
	// Create creates a new store.
	Create(ctx context.Context, s *model.Store) error
	// Update updates an existing store.
	Update(ctx context.Context, s *model.Store) error
	// Delete deletes a store by ID.
	Delete(ctx context.Context, id uuid.UUID) error
}

// EmployeeFilter holds optional filters for cross-tenant employee list queries.
// Zero values are ignored; only non-zero fields are applied.
type EmployeeFilter struct {
	// Position restricts results to a specific position (e.g. "manager", "employee").
	Position string
	// JobRole restricts results to employees with a specific job role (e.g. "pharmacist", "animator").
	JobRole string
	// StoreID restricts results to a specific store/tenant.
	StoreID *uuid.UUID
	// Status filters by onboarding state: "active" | "pending" | "unclaimed".
	Status string
	// Integrity filters for data-quality issues: "noStore" | "noJobRole" | "expiredTokens".
	Integrity string
}

// EmployeeRepository defines operations for employee management.
type EmployeeRepository interface {
	// GetByID retrieves an employee by tenant and ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Employee, error)
	// GetByAuthID retrieves an employee by their authentication ID (global lookup).
	GetByAuthID(ctx context.Context, authID string) (*model.Employee, error)
	// GetByClaimToken retrieves an unclaimed employee by their one-time invite token (global lookup).
	GetByClaimToken(ctx context.Context, token string) (*model.Employee, error)
	// GetByEmail retrieves an unlinked employee (auth_id='') matching the given email (global lookup).
	// Returns nil, nil when not found. Used for auto-linking on first login.
	GetByEmail(ctx context.Context, email string) (*model.Employee, error)
	// List retrieves all employees for a tenant with pagination.
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error)
	// ListAll retrieves employees cross-tenant with optional filtering and pagination.
	// Results include the StoreName virtual field populated via a LEFT JOIN on stores.
	ListAll(ctx context.Context, filter EmployeeFilter, page, pageSize int) ([]*model.Employee, int64, error)
	// GetByIDGlobal retrieves any employee by ID regardless of tenant.
	GetByIDGlobal(ctx context.Context, id uuid.UUID) (*model.Employee, error)
	// DeleteGlobal soft-deletes any employee by ID regardless of tenant.
	DeleteGlobal(ctx context.Context, id uuid.UUID) error
	// Create creates a new employee.
	Create(ctx context.Context, e *model.Employee) error
	// Update updates an existing employee.
	Update(ctx context.Context, e *model.Employee) error
	// Delete deletes an employee by tenant and ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// ContractRepository defines operations for contract management.
type ContractRepository interface {
	// GetByEmployeeID retrieves the contract for an employee.
	GetByEmployeeID(ctx context.Context, tenantID, employeeID uuid.UUID) (*model.Contract, error)
	// Create creates a new contract.
	Create(ctx context.Context, c *model.Contract) error
	// Update updates an existing contract.
	Update(ctx context.Context, c *model.Contract) error
}

// WeekTemplateRepository defines operations for week template management.
type WeekTemplateRepository interface {
	// GetByEmployee retrieves all templates for an employee.
	GetByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.WeekTemplate, error)
	// UpsertForEmployee replaces all templates for an employee.
	UpsertForEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, templates []*model.WeekTemplate) error
	// DeleteByEmployee deletes all templates for an employee.
	DeleteByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) error
}

// CoverageRequirementRepository defines operations for coverage requirement management.
type CoverageRequirementRepository interface {
	// List retrieves all coverage requirements for a tenant.
	List(ctx context.Context, tenantID uuid.UUID) ([]*model.CoverageRequirement, error)
	// Create creates a new coverage requirement.
	Create(ctx context.Context, cr *model.CoverageRequirement) error
	// Update updates an existing coverage requirement.
	Update(ctx context.Context, cr *model.CoverageRequirement) error
	// Delete deletes a coverage requirement by tenant and ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// ShiftInstanceRepository defines operations for shift instance management.
type ShiftInstanceRepository interface {
	// GetByID retrieves a shift by tenant and ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error)
	// ListByDateRange retrieves shifts within a date range with pagination.
	ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error)
	// ListByDate retrieves all shifts for a specific date.
	ListByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error)
	// Create creates a new shift.
	Create(ctx context.Context, s *model.ShiftInstance) error
	// CreateBatch creates multiple shifts in a batch.
	CreateBatch(ctx context.Context, shifts []*model.ShiftInstance) error
	// Update updates an existing shift.
	Update(ctx context.Context, s *model.ShiftInstance) error
	// Delete deletes a shift by tenant and ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	// DeleteBySourceTemplate deletes all template-sourced shifts within a date range.
	DeleteBySourceTemplate(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error
	// DeleteByDateRange deletes ALL shifts (any source) within a date range.
	// Used by the force-regenerate action to clear stale manual shifts before re-projecting.
	DeleteByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error
	// DeleteBySlotIDs deletes slot-sourced shifts whose SourceTemplateID is in
	// slotIDs within the given date range. Used by PublishScheme to clear stale shifts.
	DeleteBySlotIDs(ctx context.Context, tenantID uuid.UUID, slotIDs []uuid.UUID, from, to time.Time) error
	// ListByIDs retrieves multiple shifts by their IDs in a single query.
	ListByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*model.ShiftInstance, error)
	// SetStatusByDateRange bulk-updates the status of all shifts in a date range.
	// Returns the number of rows affected.
	SetStatusByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time, status string) (int64, error)
}

// ShiftAssignmentRepository defines operations for shift assignment management.
type ShiftAssignmentRepository interface {
	// GetByID retrieves an assignment by tenant and ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftAssignment, error)
	// ListByShift retrieves all assignments for a shift.
	ListByShift(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error)
	// ListByDateRange retrieves all assignments for a tenant within a date range (inclusive).
	ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error)
	// ListByEmployee retrieves all assignments for an employee within a date range.
	ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error)
	// CountByShift counts confirmed assignments for a shift.
	CountByShift(ctx context.Context, tenantID, shiftID uuid.UUID) (int64, error)
	// Create creates a new assignment.
	Create(ctx context.Context, a *model.ShiftAssignment) error
	// CreateBatch inserts all assignments in a single SQL statement.
	CreateBatch(ctx context.Context, assignments []*model.ShiftAssignment) error
	// Update updates an existing assignment.
	Update(ctx context.Context, a *model.ShiftAssignment) error
	// Delete deletes an assignment by tenant and ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
	// DeleteByDateRange deletes all assignments for a tenant whose shift_date falls
	// within [from, to] inclusive. Used by the "reset week" feature.
	DeleteByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error)
	// ExistsConflict checks if an employee has conflicting assignments on the same date with overlapping times.
	ExistsConflict(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time, startTime, endTime string, excludeAssignmentID *uuid.UUID) (bool, error)
}

// AvailabilityRepository defines operations for availability management.
type AvailabilityRepository interface {
	// GetByEmployeeDate retrieves availability for an employee on a specific date.
	GetByEmployeeDate(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time) (*model.Availability, error)
	// ListByEmployee retrieves all availability records for an employee within a date range.
	ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.Availability, error)
	// Upsert inserts or updates an availability record.
	Upsert(ctx context.Context, a *model.Availability) error
}

// LeaveRequestRepository defines operations for leave request management.
type LeaveRequestRepository interface {
	// GetByID retrieves a leave request by tenant and ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.LeaveRequest, error)
	// ListByEmployee retrieves leave requests for an employee with pagination.
	ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error)
	// ListByStore retrieves leave requests for a tenant with pagination, optionally filtered by status.
	ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.LeaveRequest, int64, error)
	// HasActiveLeave checks if an employee has any approved leaves overlapping a date range.
	HasActiveLeave(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (bool, error)
	// Create creates a new leave request.
	Create(ctx context.Context, lr *model.LeaveRequest) error
	// Update updates an existing leave request.
	Update(ctx context.Context, lr *model.LeaveRequest) error
	// Delete removes a leave request by tenant and ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// ShiftSlotRepository defines CRUD operations for store-level shift slot templates.
type ShiftSlotRepository interface {
	// List returns all shift slots for a tenant/store.
	List(ctx context.Context, tenantID uuid.UUID) ([]*model.ShiftSlot, error)
	// GetByID returns a single slot by ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftSlot, error)
	// ListByScheme returns all slots for a given scheme (A or B).
	ListByScheme(ctx context.Context, tenantID uuid.UUID, scheme string) ([]*model.ShiftSlot, error)
	// Create persists a new slot.
	Create(ctx context.Context, slot *model.ShiftSlot) error
	// Delete removes a slot by ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// AuditLogFilter holds optional filters for audit log list queries.
type AuditLogFilter struct {
	Action       string     // e.g. "leave.updated"
	ResourceType string     // e.g. "leave_request"
	ActorID      *uuid.UUID // filter by a specific actor
	From         *time.Time // inclusive lower bound on created_at
	To           *time.Time // inclusive upper bound on created_at
}

// AuditLogRepository defines operations for audit log queries.
type AuditLogRepository interface {
	// List retrieves paginated audit logs for a tenant with optional filters.
	List(ctx context.Context, tenantID uuid.UUID, filter AuditLogFilter, page, pageSize int) ([]*model.AuditLog, int64, error)
}

// RuleRepository defines CRUD operations for scheduling rules.
type RuleRepository interface {
	// List retrieves all enabled and disabled rules for a tenant.
	List(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error)
	// ListEnabled retrieves only enabled rules for a tenant (used by the rule engine).
	ListEnabled(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error)
	// GetByID retrieves a rule by its ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Rule, error)
	// Create creates a new rule.
	Create(ctx context.Context, r *model.Rule) error
	// Update updates an existing rule.
	Update(ctx context.Context, r *model.Rule) error
	// Delete soft-deletes a rule.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// AIInsightRepository defines operations for AI-generated insights.
type AIInsightRepository interface {
	// List retrieves active (non-dismissed) insights for a tenant.
	List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.AIInsight, int64, error)
	// Create persists a new insight.
	Create(ctx context.Context, i *model.AIInsight) error
	// Dismiss marks an insight as dismissed.
	Dismiss(ctx context.Context, tenantID, id uuid.UUID) error
}

// SwapRequestRepository defines operations for swap request management.
type SwapRequestRepository interface {
	// GetByID retrieves a swap request by tenant and ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.SwapRequest, error)
	// ListByEmployee retrieves swap requests for an employee with pagination.
	ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.SwapRequest, int64, error)
	// ListByStore retrieves swap requests for a tenant with pagination, optionally filtered by status.
	ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.SwapRequest, int64, error)
	// Create creates a new swap request.
	Create(ctx context.Context, sr *model.SwapRequest) error
	// Update updates an existing swap request.
	Update(ctx context.Context, sr *model.SwapRequest) error
}

// SchedulePlanRepository defines the lifecycle state for a week-level plan.
type SchedulePlanRepository interface {
	// GetByWeekStart returns the plan for a store/week (nil, nil if not found).
	GetByWeekStart(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (*model.SchedulePlan, error)
	// GetByID returns the plan by ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error)
	// Create persists a new plan.
	Create(ctx context.Context, p *model.SchedulePlan) error
	// Update persists changes to an existing plan.
	Update(ctx context.Context, p *model.SchedulePlan) error
}

// QualificationRepository defines operations for qualification management.
type QualificationRepository interface {
	// List retrieves all qualifications for a tenant.
	List(ctx context.Context, tenantID uuid.UUID) ([]*model.Qualification, error)
	// GetByID retrieves a qualification by ID.
	GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Qualification, error)
	// Create persists a new qualification.
	Create(ctx context.Context, q *model.Qualification) error
	// Update persists changes to a qualification.
	Update(ctx context.Context, q *model.Qualification) error
	// Delete removes a qualification.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// EmployeeQualificationRepository defines operations for employee qualifications.
type EmployeeQualificationRepository interface {
	// ListByEmployee retrieves all qualifications held by an employee.
	ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.EmployeeQualification, error)
	// ListExpiringWithinDays retrieves qualifications expiring within N days.
	ListExpiringWithinDays(ctx context.Context, tenantID uuid.UUID, days int) ([]*model.EmployeeQualification, error)
	// HasValidQualification checks if an employee holds a valid (non-expired) qualification.
	HasValidQualification(ctx context.Context, tenantID, employeeID, qualificationID uuid.UUID) (bool, error)
	// Create persists a new employee qualification.
	Create(ctx context.Context, eq *model.EmployeeQualification) error
	// Update persists changes to an employee qualification.
	Update(ctx context.Context, eq *model.EmployeeQualification) error
	// Delete removes an employee qualification.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}

// PublicHolidayRepository defines operations for public holiday reference data.
type PublicHolidayRepository interface {
	// ListByYear returns all holidays for the given year and zone.
	ListByYear(ctx context.Context, year int, zone string) ([]*model.PublicHoliday, error)
	// GetByDate returns the holiday on a specific date for a zone, or nil if not a holiday.
	GetByDate(ctx context.Context, date time.Time, zone string) (*model.PublicHoliday, error)
	// UpsertBatch inserts or updates a batch of holidays (used when syncing from the API).
	UpsertBatch(ctx context.Context, holidays []*model.PublicHoliday) error
}

// PlanningModelMetricRepository defines operations for planning model metrics.
type PlanningModelMetricRepository interface {
	// ListByScheme retrieves the most recent N weeks of metrics for a model scheme.
	ListByScheme(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) ([]*model.PlanningModelMetric, error)
	// Upsert inserts or updates a metric by (tenant_id, model_scheme, week_start).
	Upsert(ctx context.Context, m *model.PlanningModelMetric) error
}

// StoreExceptionRepository defines operations for store-level schedule exceptions.
type StoreExceptionRepository interface {
	// ListByDateRange returns all exceptions for the store within [from, to] inclusive.
	ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.StoreException, error)
	// Create persists a new exception.
	Create(ctx context.Context, e *model.StoreException) error
	// Delete removes an exception by ID.
	Delete(ctx context.Context, tenantID, id uuid.UUID) error
}
