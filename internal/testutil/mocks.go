// Package testutil provides mock repository implementations and test helpers
// for ParaShift service and handler unit tests.
package testutil

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
)

// ─── StoreRepository mock ─────────────────────────────────────────────────────

type MockStoreRepo struct {
	GetByIDFn func(ctx context.Context, id uuid.UUID) (*model.Store, error)
	ListFn    func(ctx context.Context, page, pageSize int) ([]*model.Store, int64, error)
	CreateFn  func(ctx context.Context, s *model.Store) error
	UpdateFn  func(ctx context.Context, s *model.Store) error
	DeleteFn  func(ctx context.Context, id uuid.UUID) error
}

func (m *MockStoreRepo) GetByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, id)
	}
	return nil, nil
}
func (m *MockStoreRepo) List(ctx context.Context, page, pageSize int) ([]*model.Store, int64, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockStoreRepo) Create(ctx context.Context, s *model.Store) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, s)
	}
	return nil
}
func (m *MockStoreRepo) Update(ctx context.Context, s *model.Store) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, s)
	}
	return nil
}
func (m *MockStoreRepo) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, id)
	}
	return nil
}

// ─── EmployeeRepository mock ──────────────────────────────────────────────────

type MockEmployeeRepo struct {
	GetByIDFn         func(ctx context.Context, tenantID, id uuid.UUID) (*model.Employee, error)
	GetByAuthIDFn     func(ctx context.Context, authID string) (*model.Employee, error)
	GetByEmailFn      func(ctx context.Context, email string) (*model.Employee, error)
	GetByClaimTokenFn func(ctx context.Context, token string) (*model.Employee, error)
	ListFn            func(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error)
	ListAllFn         func(ctx context.Context, filter repo.EmployeeFilter, page, pageSize int) ([]*model.Employee, int64, error)
	GetByIDGlobalFn   func(ctx context.Context, id uuid.UUID) (*model.Employee, error)
	DeleteGlobalFn    func(ctx context.Context, id uuid.UUID) error
	CreateFn          func(ctx context.Context, e *model.Employee) error
	UpdateFn          func(ctx context.Context, e *model.Employee) error
	DeleteFn          func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockEmployeeRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Employee, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockEmployeeRepo) GetByAuthID(ctx context.Context, authID string) (*model.Employee, error) {
	if m.GetByAuthIDFn != nil {
		return m.GetByAuthIDFn(ctx, authID)
	}
	return nil, nil
}
func (m *MockEmployeeRepo) GetByEmail(ctx context.Context, email string) (*model.Employee, error) {
	if m.GetByEmailFn != nil {
		return m.GetByEmailFn(ctx, email)
	}
	return nil, nil
}
func (m *MockEmployeeRepo) GetByClaimToken(ctx context.Context, token string) (*model.Employee, error) {
	if m.GetByClaimTokenFn != nil {
		return m.GetByClaimTokenFn(ctx, token)
	}
	return nil, nil
}
func (m *MockEmployeeRepo) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockEmployeeRepo) ListAll(ctx context.Context, filter repo.EmployeeFilter, page, pageSize int) ([]*model.Employee, int64, error) {
	if m.ListAllFn != nil {
		return m.ListAllFn(ctx, filter, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockEmployeeRepo) GetByIDGlobal(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	if m.GetByIDGlobalFn != nil {
		return m.GetByIDGlobalFn(ctx, id)
	}
	return nil, nil
}
func (m *MockEmployeeRepo) DeleteGlobal(ctx context.Context, id uuid.UUID) error {
	if m.DeleteGlobalFn != nil {
		return m.DeleteGlobalFn(ctx, id)
	}
	return nil
}
func (m *MockEmployeeRepo) Create(ctx context.Context, e *model.Employee) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, e)
	}
	return nil
}
func (m *MockEmployeeRepo) Update(ctx context.Context, e *model.Employee) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, e)
	}
	return nil
}
func (m *MockEmployeeRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── ContractRepository mock ──────────────────────────────────────────────────

type MockContractRepo struct {
	GetByEmployeeIDFn func(ctx context.Context, tenantID, employeeID uuid.UUID) (*model.Contract, error)
	CreateFn          func(ctx context.Context, c *model.Contract) error
	UpdateFn          func(ctx context.Context, c *model.Contract) error
}

func (m *MockContractRepo) GetByEmployeeID(ctx context.Context, tenantID, employeeID uuid.UUID) (*model.Contract, error) {
	if m.GetByEmployeeIDFn != nil {
		return m.GetByEmployeeIDFn(ctx, tenantID, employeeID)
	}
	return nil, nil
}
func (m *MockContractRepo) Create(ctx context.Context, c *model.Contract) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, c)
	}
	return nil
}
func (m *MockContractRepo) Update(ctx context.Context, c *model.Contract) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, c)
	}
	return nil
}

// ─── WeekTemplateRepository mock ─────────────────────────────────────────────

type MockWeekTemplateRepo struct {
	GetByEmployeeFn      func(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.WeekTemplate, error)
	UpsertForEmployeeFn  func(ctx context.Context, tenantID, employeeID uuid.UUID, templates []*model.WeekTemplate) error
	DeleteByEmployeeFn   func(ctx context.Context, tenantID, employeeID uuid.UUID) error
}

func (m *MockWeekTemplateRepo) GetByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.WeekTemplate, error) {
	if m.GetByEmployeeFn != nil {
		return m.GetByEmployeeFn(ctx, tenantID, employeeID)
	}
	return nil, nil
}
func (m *MockWeekTemplateRepo) UpsertForEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, templates []*model.WeekTemplate) error {
	if m.UpsertForEmployeeFn != nil {
		return m.UpsertForEmployeeFn(ctx, tenantID, employeeID, templates)
	}
	return nil
}
func (m *MockWeekTemplateRepo) DeleteByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) error {
	if m.DeleteByEmployeeFn != nil {
		return m.DeleteByEmployeeFn(ctx, tenantID, employeeID)
	}
	return nil
}

// ─── CoverageRequirementRepository mock ──────────────────────────────────────

type MockCoverageRequirementRepo struct {
	ListFn   func(ctx context.Context, tenantID uuid.UUID) ([]*model.CoverageRequirement, error)
	CreateFn func(ctx context.Context, cr *model.CoverageRequirement) error
	UpdateFn func(ctx context.Context, cr *model.CoverageRequirement) error
	DeleteFn func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockCoverageRequirementRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*model.CoverageRequirement, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *MockCoverageRequirementRepo) Create(ctx context.Context, cr *model.CoverageRequirement) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, cr)
	}
	return nil
}
func (m *MockCoverageRequirementRepo) Update(ctx context.Context, cr *model.CoverageRequirement) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, cr)
	}
	return nil
}
func (m *MockCoverageRequirementRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── ShiftInstanceRepository mock ────────────────────────────────────────────

type MockShiftInstanceRepo struct {
	GetByIDFn                func(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error)
	ListByDateRangeFn        func(ctx context.Context, tenantID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error)
	ListByDateFn             func(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error)
	CreateFn                 func(ctx context.Context, s *model.ShiftInstance) error
	CreateBatchFn            func(ctx context.Context, shifts []*model.ShiftInstance) error
	UpdateFn                 func(ctx context.Context, s *model.ShiftInstance) error
	DeleteFn                 func(ctx context.Context, tenantID, id uuid.UUID) error
	DeleteBySourceTemplateFn func(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error
	DeleteBySlotIDsFn        func(ctx context.Context, tenantID uuid.UUID, slotIDs []uuid.UUID, from, to time.Time) error
	ListByIDsFn              func(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*model.ShiftInstance, error)
	SetStatusByDateRangeFn   func(ctx context.Context, tenantID uuid.UUID, from, to time.Time, status string) (int64, error)
	DeleteByDateRangeFn      func(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error
	DeleteByIDsFn            func(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error
}

func (m *MockShiftInstanceRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftInstance, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockShiftInstanceRepo) ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time, page, pageSize int) ([]*model.ShiftInstance, int64, error) {
	if m.ListByDateRangeFn != nil {
		return m.ListByDateRangeFn(ctx, tenantID, from, to, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockShiftInstanceRepo) ListByDate(ctx context.Context, tenantID uuid.UUID, date time.Time) ([]*model.ShiftInstance, error) {
	if m.ListByDateFn != nil {
		return m.ListByDateFn(ctx, tenantID, date)
	}
	return nil, nil
}
func (m *MockShiftInstanceRepo) Create(ctx context.Context, s *model.ShiftInstance) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, s)
	}
	return nil
}
func (m *MockShiftInstanceRepo) CreateBatch(ctx context.Context, shifts []*model.ShiftInstance) error {
	if m.CreateBatchFn != nil {
		return m.CreateBatchFn(ctx, shifts)
	}
	return nil
}
func (m *MockShiftInstanceRepo) Update(ctx context.Context, s *model.ShiftInstance) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, s)
	}
	return nil
}
func (m *MockShiftInstanceRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}
func (m *MockShiftInstanceRepo) DeleteBySourceTemplate(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error {
	if m.DeleteBySourceTemplateFn != nil {
		return m.DeleteBySourceTemplateFn(ctx, tenantID, from, to)
	}
	return nil
}
func (m *MockShiftInstanceRepo) DeleteBySlotIDs(ctx context.Context, tenantID uuid.UUID, slotIDs []uuid.UUID, from, to time.Time) error {
	if m.DeleteBySlotIDsFn != nil {
		return m.DeleteBySlotIDsFn(ctx, tenantID, slotIDs, from, to)
	}
	return nil
}
func (m *MockShiftInstanceRepo) ListByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) ([]*model.ShiftInstance, error) {
	if m.ListByIDsFn != nil {
		return m.ListByIDsFn(ctx, tenantID, ids)
	}
	return nil, nil
}

func (m *MockShiftInstanceRepo) DeleteByIDs(ctx context.Context, tenantID uuid.UUID, ids []uuid.UUID) error {
	if m.DeleteByIDsFn != nil {
		return m.DeleteByIDsFn(ctx, tenantID, ids)
	}
	return nil
}
func (m *MockShiftInstanceRepo) SetStatusByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time, status string) (int64, error) {
	if m.SetStatusByDateRangeFn != nil {
		return m.SetStatusByDateRangeFn(ctx, tenantID, from, to, status)
	}
	return 0, nil
}
func (m *MockShiftInstanceRepo) DeleteByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) error {
	if m.DeleteByDateRangeFn != nil {
		return m.DeleteByDateRangeFn(ctx, tenantID, from, to)
	}
	return nil
}

// ─── ShiftAssignmentRepository mock ──────────────────────────────────────────

type MockShiftAssignmentRepo struct {
	GetByIDFn              func(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftAssignment, error)
	ListByDateRangeFn      func(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error)
	ListByShiftFn          func(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error)
	ListByEmployeeFn       func(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error)
	CountByShiftFn         func(ctx context.Context, tenantID, shiftID uuid.UUID) (int64, error)
	CreateFn               func(ctx context.Context, a *model.ShiftAssignment) error
	CreateBatchFn          func(ctx context.Context, assignments []*model.ShiftAssignment) error
	UpdateFn               func(ctx context.Context, a *model.ShiftAssignment) error
	DeleteFn               func(ctx context.Context, tenantID, id uuid.UUID) error
	DeleteByDateRangeFn    func(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error)
	ExistsConflictFn       func(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time, startTime, endTime string, excludeAssignmentID *uuid.UUID) (bool, error)
}

func (m *MockShiftAssignmentRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftAssignment, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockShiftAssignmentRepo) ListByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	if m.ListByDateRangeFn != nil {
		return m.ListByDateRangeFn(ctx, tenantID, from, to)
	}
	return nil, nil
}
func (m *MockShiftAssignmentRepo) ListByShift(ctx context.Context, tenantID, shiftID uuid.UUID) ([]*model.ShiftAssignment, error) {
	if m.ListByShiftFn != nil {
		return m.ListByShiftFn(ctx, tenantID, shiftID)
	}
	return nil, nil
}
func (m *MockShiftAssignmentRepo) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.ShiftAssignment, error) {
	if m.ListByEmployeeFn != nil {
		return m.ListByEmployeeFn(ctx, tenantID, employeeID, from, to)
	}
	return nil, nil
}
func (m *MockShiftAssignmentRepo) CountByShift(ctx context.Context, tenantID, shiftID uuid.UUID) (int64, error) {
	if m.CountByShiftFn != nil {
		return m.CountByShiftFn(ctx, tenantID, shiftID)
	}
	return 0, nil
}
func (m *MockShiftAssignmentRepo) Create(ctx context.Context, a *model.ShiftAssignment) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, a)
	}
	return nil
}
func (m *MockShiftAssignmentRepo) CreateBatch(ctx context.Context, assignments []*model.ShiftAssignment) error {
	if m.CreateBatchFn != nil {
		return m.CreateBatchFn(ctx, assignments)
	}
	return nil
}
func (m *MockShiftAssignmentRepo) Update(ctx context.Context, a *model.ShiftAssignment) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, a)
	}
	return nil
}
func (m *MockShiftAssignmentRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}
func (m *MockShiftAssignmentRepo) DeleteByDateRange(ctx context.Context, tenantID uuid.UUID, from, to time.Time) (int64, error) {
	if m.DeleteByDateRangeFn != nil {
		return m.DeleteByDateRangeFn(ctx, tenantID, from, to)
	}
	return 0, nil
}
func (m *MockShiftAssignmentRepo) ExistsConflict(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time, startTime, endTime string, excludeAssignmentID *uuid.UUID) (bool, error) {
	if m.ExistsConflictFn != nil {
		return m.ExistsConflictFn(ctx, tenantID, employeeID, date, startTime, endTime, excludeAssignmentID)
	}
	return false, nil
}

// ─── AvailabilityRepository mock ─────────────────────────────────────────────

type MockAvailabilityRepo struct {
	GetByEmployeeDateFn func(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time) (*model.Availability, error)
	ListByEmployeeFn    func(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.Availability, error)
	UpsertFn            func(ctx context.Context, a *model.Availability) error
}

func (m *MockAvailabilityRepo) GetByEmployeeDate(ctx context.Context, tenantID, employeeID uuid.UUID, date time.Time) (*model.Availability, error) {
	if m.GetByEmployeeDateFn != nil {
		return m.GetByEmployeeDateFn(ctx, tenantID, employeeID, date)
	}
	return nil, nil
}
func (m *MockAvailabilityRepo) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) ([]*model.Availability, error) {
	if m.ListByEmployeeFn != nil {
		return m.ListByEmployeeFn(ctx, tenantID, employeeID, from, to)
	}
	return nil, nil
}
func (m *MockAvailabilityRepo) Upsert(ctx context.Context, a *model.Availability) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, a)
	}
	return nil
}

// ─── LeaveRequestRepository mock ─────────────────────────────────────────────

type MockLeaveRequestRepo struct {
	GetByIDFn         func(ctx context.Context, tenantID, id uuid.UUID) (*model.LeaveRequest, error)
	ListByEmployeeFn  func(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error)
	ListByStoreFn     func(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.LeaveRequest, int64, error)
	HasActiveLeaveFn  func(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (bool, error)
	CreateFn          func(ctx context.Context, lr *model.LeaveRequest) error
	UpdateFn          func(ctx context.Context, lr *model.LeaveRequest) error
	DeleteFn          func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockLeaveRequestRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.LeaveRequest, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockLeaveRequestRepo) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
	if m.ListByEmployeeFn != nil {
		return m.ListByEmployeeFn(ctx, tenantID, employeeID, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockLeaveRequestRepo) ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.LeaveRequest, int64, error) {
	if m.ListByStoreFn != nil {
		return m.ListByStoreFn(ctx, tenantID, status, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockLeaveRequestRepo) HasActiveLeave(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (bool, error) {
	if m.HasActiveLeaveFn != nil {
		return m.HasActiveLeaveFn(ctx, tenantID, employeeID, from, to)
	}
	return false, nil
}
func (m *MockLeaveRequestRepo) Create(ctx context.Context, lr *model.LeaveRequest) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, lr)
	}
	return nil
}
func (m *MockLeaveRequestRepo) Update(ctx context.Context, lr *model.LeaveRequest) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, lr)
	}
	return nil
}
func (m *MockLeaveRequestRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── ShiftSlotRepository mock ─────────────────────────────────────────────────

type MockShiftSlotRepo struct {
	ListFn          func(ctx context.Context, tenantID uuid.UUID) ([]*model.ShiftSlot, error)
	GetByIDFn       func(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftSlot, error)
	ListBySchemeFn  func(ctx context.Context, tenantID uuid.UUID, scheme string) ([]*model.ShiftSlot, error)
	CreateFn        func(ctx context.Context, slot *model.ShiftSlot) error
	DeleteFn        func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockShiftSlotRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*model.ShiftSlot, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *MockShiftSlotRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.ShiftSlot, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockShiftSlotRepo) ListByScheme(ctx context.Context, tenantID uuid.UUID, scheme string) ([]*model.ShiftSlot, error) {
	if m.ListBySchemeFn != nil {
		return m.ListBySchemeFn(ctx, tenantID, scheme)
	}
	return nil, nil
}
func (m *MockShiftSlotRepo) Create(ctx context.Context, slot *model.ShiftSlot) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, slot)
	}
	return nil
}
func (m *MockShiftSlotRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── SwapRequestRepository mock ──────────────────────────────────────────────

type MockSwapRequestRepo struct {
	GetByIDFn       func(ctx context.Context, tenantID, id uuid.UUID) (*model.SwapRequest, error)
	ListByEmployeeFn func(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.SwapRequest, int64, error)
	ListByStoreFn   func(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.SwapRequest, int64, error)
	CreateFn        func(ctx context.Context, sr *model.SwapRequest) error
	UpdateFn        func(ctx context.Context, sr *model.SwapRequest) error
}

func (m *MockSwapRequestRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.SwapRequest, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockSwapRequestRepo) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, page, pageSize int) ([]*model.SwapRequest, int64, error) {
	if m.ListByEmployeeFn != nil {
		return m.ListByEmployeeFn(ctx, tenantID, employeeID, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockSwapRequestRepo) ListByStore(ctx context.Context, tenantID uuid.UUID, status string, page, pageSize int) ([]*model.SwapRequest, int64, error) {
	if m.ListByStoreFn != nil {
		return m.ListByStoreFn(ctx, tenantID, status, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockSwapRequestRepo) Create(ctx context.Context, sr *model.SwapRequest) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, sr)
	}
	return nil
}
func (m *MockSwapRequestRepo) Update(ctx context.Context, sr *model.SwapRequest) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, sr)
	}
	return nil
}

// ─── AuditLogRepository mock ──────────────────────────────────────────────────

type MockAuditLogRepo struct {
	ListFn func(ctx context.Context, tenantID uuid.UUID, filter repo.AuditLogFilter, page, pageSize int) ([]*model.AuditLog, int64, error)
}

func (m *MockAuditLogRepo) List(ctx context.Context, tenantID uuid.UUID, filter repo.AuditLogFilter, page, pageSize int) ([]*model.AuditLog, int64, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID, filter, page, pageSize)
	}
	return nil, 0, nil
}

// ─── RuleRepository mock ──────────────────────────────────────────────────────

type MockRuleRepo struct {
	ListFn        func(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error)
	ListEnabledFn func(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error)
	GetByIDFn     func(ctx context.Context, tenantID, id uuid.UUID) (*model.Rule, error)
	CreateFn      func(ctx context.Context, r *model.Rule) error
	UpdateFn      func(ctx context.Context, r *model.Rule) error
	DeleteFn      func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockRuleRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *MockRuleRepo) ListEnabled(ctx context.Context, tenantID uuid.UUID) ([]*model.Rule, error) {
	if m.ListEnabledFn != nil {
		return m.ListEnabledFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *MockRuleRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Rule, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockRuleRepo) Create(ctx context.Context, r *model.Rule) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, r)
	}
	return nil
}
func (m *MockRuleRepo) Update(ctx context.Context, r *model.Rule) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, r)
	}
	return nil
}
func (m *MockRuleRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── AIInsightRepository mock ─────────────────────────────────────────────────

type MockAIInsightRepo struct {
	ListFn    func(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.AIInsight, int64, error)
	CreateFn  func(ctx context.Context, i *model.AIInsight) error
	DismissFn func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockAIInsightRepo) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.AIInsight, int64, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID, page, pageSize)
	}
	return nil, 0, nil
}
func (m *MockAIInsightRepo) Create(ctx context.Context, i *model.AIInsight) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, i)
	}
	return nil
}
func (m *MockAIInsightRepo) Dismiss(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DismissFn != nil {
		return m.DismissFn(ctx, tenantID, id)
	}
	return nil
}

// ─── SchedulePlanRepository mock ─────────────────────────────────────────────

type MockSchedulePlanRepo struct {
	GetByWeekStartFn func(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (*model.SchedulePlan, error)
	GetByIDFn        func(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error)
	CreateFn         func(ctx context.Context, p *model.SchedulePlan) error
	UpdateFn         func(ctx context.Context, p *model.SchedulePlan) error
}

func (m *MockSchedulePlanRepo) GetByWeekStart(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (*model.SchedulePlan, error) {
	if m.GetByWeekStartFn != nil {
		return m.GetByWeekStartFn(ctx, tenantID, weekStart)
	}
	return nil, nil
}
func (m *MockSchedulePlanRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.SchedulePlan, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockSchedulePlanRepo) Create(ctx context.Context, p *model.SchedulePlan) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, p)
	}
	return nil
}
func (m *MockSchedulePlanRepo) Update(ctx context.Context, p *model.SchedulePlan) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, p)
	}
	return nil
}

// ─── QualificationRepository mock ────────────────────────────────────────────

type MockQualificationRepo struct {
	ListFn   func(ctx context.Context, tenantID uuid.UUID) ([]*model.Qualification, error)
	GetByIDFn func(ctx context.Context, tenantID, id uuid.UUID) (*model.Qualification, error)
	CreateFn func(ctx context.Context, q *model.Qualification) error
	UpdateFn func(ctx context.Context, q *model.Qualification) error
	DeleteFn func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockQualificationRepo) List(ctx context.Context, tenantID uuid.UUID) ([]*model.Qualification, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, tenantID)
	}
	return nil, nil
}
func (m *MockQualificationRepo) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Qualification, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, tenantID, id)
	}
	return nil, nil
}
func (m *MockQualificationRepo) Create(ctx context.Context, q *model.Qualification) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, q)
	}
	return nil
}
func (m *MockQualificationRepo) Update(ctx context.Context, q *model.Qualification) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, q)
	}
	return nil
}
func (m *MockQualificationRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── EmployeeQualificationRepository mock ───────────────────────────────────

type MockEmployeeQualificationRepo struct {
	ListByEmployeeFn           func(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.EmployeeQualification, error)
	ListExpiringWithinDaysFn   func(ctx context.Context, tenantID uuid.UUID, days int) ([]*model.EmployeeQualification, error)
	HasValidQualificationFn    func(ctx context.Context, tenantID, employeeID, qualificationID uuid.UUID) (bool, error)
	CreateFn                   func(ctx context.Context, eq *model.EmployeeQualification) error
	UpdateFn                   func(ctx context.Context, eq *model.EmployeeQualification) error
	DeleteFn                   func(ctx context.Context, tenantID, id uuid.UUID) error
}

func (m *MockEmployeeQualificationRepo) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.EmployeeQualification, error) {
	if m.ListByEmployeeFn != nil {
		return m.ListByEmployeeFn(ctx, tenantID, employeeID)
	}
	return nil, nil
}
func (m *MockEmployeeQualificationRepo) ListExpiringWithinDays(ctx context.Context, tenantID uuid.UUID, days int) ([]*model.EmployeeQualification, error) {
	if m.ListExpiringWithinDaysFn != nil {
		return m.ListExpiringWithinDaysFn(ctx, tenantID, days)
	}
	return nil, nil
}
func (m *MockEmployeeQualificationRepo) HasValidQualification(ctx context.Context, tenantID, employeeID, qualificationID uuid.UUID) (bool, error) {
	if m.HasValidQualificationFn != nil {
		return m.HasValidQualificationFn(ctx, tenantID, employeeID, qualificationID)
	}
	return false, nil
}
func (m *MockEmployeeQualificationRepo) Create(ctx context.Context, eq *model.EmployeeQualification) error {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, eq)
	}
	return nil
}
func (m *MockEmployeeQualificationRepo) Update(ctx context.Context, eq *model.EmployeeQualification) error {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, eq)
	}
	return nil
}
func (m *MockEmployeeQualificationRepo) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, tenantID, id)
	}
	return nil
}

// ─── PublicHolidayRepository mock ────────────────────────────────────────────

type MockPublicHolidayRepo struct {
	ListByYearFn func(ctx context.Context, year int, zone string) ([]*model.PublicHoliday, error)
	GetByDateFn  func(ctx context.Context, date time.Time, zone string) (*model.PublicHoliday, error)
	UpsertBatchFn func(ctx context.Context, holidays []*model.PublicHoliday) error
}

func (m *MockPublicHolidayRepo) ListByYear(ctx context.Context, year int, zone string) ([]*model.PublicHoliday, error) {
	if m.ListByYearFn != nil {
		return m.ListByYearFn(ctx, year, zone)
	}
	return nil, nil
}
func (m *MockPublicHolidayRepo) GetByDate(ctx context.Context, date time.Time, zone string) (*model.PublicHoliday, error) {
	if m.GetByDateFn != nil {
		return m.GetByDateFn(ctx, date, zone)
	}
	return nil, nil
}
func (m *MockPublicHolidayRepo) UpsertBatch(ctx context.Context, holidays []*model.PublicHoliday) error {
	if m.UpsertBatchFn != nil {
		return m.UpsertBatchFn(ctx, holidays)
	}
	return nil
}

// MockTokenRevocationRepo is a test double for repo.TokenRevocationRepository.
type MockTokenRevocationRepo struct {
	GetBySubFn func(ctx context.Context, sub string) (*model.TokenRevocation, error)
	UpsertFn   func(ctx context.Context, sub string, revokedAfter time.Time) error
}

func (m *MockTokenRevocationRepo) GetBySub(ctx context.Context, sub string) (*model.TokenRevocation, error) {
	if m.GetBySubFn != nil {
		return m.GetBySubFn(ctx, sub)
	}
	return nil, nil
}
func (m *MockTokenRevocationRepo) Upsert(ctx context.Context, sub string, revokedAfter time.Time) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, sub, revokedAfter)
	}
	return nil
}

// ─── PlanningModelMetricRepository mock ──────────────────────────────────────

type MockPlanningModelMetricRepo struct {
	ListBySchemeFn func(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) ([]*model.PlanningModelMetric, error)
	UpsertFn       func(ctx context.Context, m *model.PlanningModelMetric) error
}

func (m *MockPlanningModelMetricRepo) ListByScheme(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) ([]*model.PlanningModelMetric, error) {
	if m.ListBySchemeFn != nil {
		return m.ListBySchemeFn(ctx, tenantID, scheme, limitWeeks)
	}
	return nil, nil
}
func (m *MockPlanningModelMetricRepo) Upsert(ctx context.Context, metric *model.PlanningModelMetric) error {
	if m.UpsertFn != nil {
		return m.UpsertFn(ctx, metric)
	}
	return nil
}

// StoreExceptionRepository is mocked via mockery — see internal/repo/mocks
// (DX-3). The hand-written mock was removed.
