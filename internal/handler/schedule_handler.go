package handler

import (
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// ScheduleHandler handles schedule-related HTTP requests.
type ScheduleHandler struct {
	svc *service.ScheduleService
}

// NewScheduleHandler creates a new ScheduleHandler.
func NewScheduleHandler(svc *service.ScheduleService) *ScheduleHandler {
	return &ScheduleHandler{svc: svc}
}

// shiftResponse mirrors model.ShiftInstance for HTTP output.
// StartTime/EndTime are HH:MM strings (not time.Time); Date is a date string.
type shiftResponse struct {
	ID                    uuid.UUID  `json:"id"`
	TenantID              uuid.UUID  `json:"tenant_id"`
	Date                  string     `json:"date"`
	StartTime             string     `json:"start_time"`
	EndTime               string     `json:"end_time"`
	Role                  string     `json:"role"`
	RequiredQualification string     `json:"required_qualification"`
	Source                string     `json:"source"`
	SourceTemplateID      *uuid.UUID `json:"source_template_id,omitempty"`
	CreatedAt             string     `json:"created_at"`
	UpdatedAt             string     `json:"updated_at"`
}

// assignmentResponse mirrors model.ShiftAssignment for HTTP output.
type assignmentResponse struct {
	ID              uuid.UUID `json:"id"`
	TenantID        uuid.UUID `json:"tenant_id"`
	ShiftInstanceID uuid.UUID `json:"shift_instance_id"` // was ShiftID — matches model field name
	EmployeeID      uuid.UUID `json:"employee_id"`
	Status          string    `json:"status"`
	AssignedBy      uuid.UUID `json:"assigned_by"`
	AssignedAt      string    `json:"assigned_at"`
	CreatedAt       string    `json:"created_at"`
}

// parseDate parses a YYYY-MM-DD string into time.Time.
func parseDate(dateStr string) (time.Time, error) {
	return time.Parse("2006-01-02", dateStr)
}

// validateStoreTenant returns true if the request's tenantID matches storeID or the user is admin.
func validateStoreTenant(tenantID, storeID uuid.UUID, role string) bool {
	return storeID == tenantID || role == "admin"
}

// shiftToResponse maps *model.ShiftInstance → shiftResponse.
func shiftToResponse(s *model.ShiftInstance) shiftResponse {
	return shiftResponse{
		ID:                    s.ID,
		TenantID:              s.TenantID,
		Date:                  s.Date.Format("2006-01-02"),
		StartTime:             s.StartTime, // string HH:MM
		EndTime:               s.EndTime,   // string HH:MM
		Role:                  s.Role,
		RequiredQualification: s.RequiredQualification,
		Source:                s.Source,
		SourceTemplateID:      s.SourceTemplateID,
		CreatedAt:             s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:             s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// assignToResponse maps *model.ShiftAssignment → assignmentResponse.
func assignToResponse(a *model.ShiftAssignment) assignmentResponse {
	return assignmentResponse{
		ID:              a.ID,
		TenantID:        a.TenantID,
		ShiftInstanceID: a.ShiftInstanceID, // model field name
		EmployeeID:      a.EmployeeID,
		Status:          a.Status,
		AssignedBy:      a.AssignedBy,
		AssignedAt:      a.AssignedAt.Format("2006-01-02T15:04:05Z07:00"),
		CreatedAt:       a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

// GetSchedule returns the schedule for a store within a date range.
func (h *ScheduleHandler) GetSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		pkg.WriteError(w, apierror.BadRequest("from and to query parameters are required").WithKey("errors.missingParams"))
		return
	}

	from, err := parseDate(fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	to, err := parseDate(toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	params := pagination.Parse(r)

	// Service: GetSchedule(ctx, tenantID, from, to, page, pageSize) → ([]*model.ShiftInstance, int64, error)
	shifts, total, err := h.svc.GetSchedule(ctx, storeID, from, to, params.Page, params.PerPage)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]shiftResponse, len(shifts))
	for i, s := range shifts {
		responses[i] = shiftToResponse(s)
	}

	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(responses, params, total))
}

// weekOf parses a week_of=YYYY-MM-DD query param and returns Monday 00:00 and Sunday 23:59:59.
func weekOf(r *http.Request) (from, to time.Time, err error) {
	weekStr := r.URL.Query().Get("week_of")
	if weekStr == "" {
		err = fmt.Errorf("week_of query parameter is required")
		return
	}
	from, err = time.Parse("2006-01-02", weekStr)
	if err != nil {
		err = fmt.Errorf("invalid week_of format, use YYYY-MM-DD")
		return
	}
	// Snap to Monday of that week.
	dayOfWeek := int(from.Weekday())
	if dayOfWeek == 0 {
		dayOfWeek = 7 // Sunday → 7
	}
	from = from.AddDate(0, 0, -(dayOfWeek - 1))
	to = from.AddDate(0, 0, 6)
	return
}

// ListShifts returns all shifts for a store for the given week (week_of=YYYY-MM-DD).
// Used by the Planner to populate the weekly calendar.
func (h *ScheduleHandler) ListShifts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	from, to, err := weekOf(r)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest(err.Error()).WithKey("errors.invalidInput"))
		return
	}

	// No pagination — return the full week (at most 7×N shifts, never huge).
	shifts, _, err := h.svc.GetSchedule(ctx, storeID, from, to, 1, 500)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.ShiftInstanceResponse, len(shifts))
	for i, s := range shifts {
		responses[i] = dto.ShiftInstanceResponse{
			ID:            s.ID,
			StoreID:       s.TenantID,
			Date:          s.Date.Format("2006-01-02"),
			StartTime:     s.StartTime,
			EndTime:       s.EndTime,
			Role:          s.Role,
			RequiredCount: 1, // default; no per-shift head-count stored yet
			Status:        s.Status,
			Source:        s.Source,
			TemplateID:    s.SourceTemplateID,
			NeedsCover:    s.NeedsCover,
			CreatedAt:     s.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:     s.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	pkg.WriteJSON(w, http.StatusOK, responses)
}

// ListAssignments returns all assignments for a store for the given week (week_of=YYYY-MM-DD).
// Used by the Planner alongside ListShifts to populate the assignment grid.
func (h *ScheduleHandler) ListAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	from, to, err := weekOf(r)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest(err.Error()).WithKey("errors.invalidInput"))
		return
	}

	assignments, err := h.svc.GetAssignmentsByDateRange(ctx, storeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.AssignmentResponse, len(assignments))
	for i, a := range assignments {
		responses[i] = dto.AssignmentResponse{
			ID:             a.ID,
			StoreID:        a.TenantID,
			ShiftID:        a.ShiftInstanceID,
			EmployeeID:     a.EmployeeID,
			Status:         a.Status,
			ShiftDate:      a.ShiftDate.Format("2006-01-02"),
			ShiftStartTime: a.ShiftStartTime,
			ShiftEndTime:   a.ShiftEndTime,
			Violations:     []dto.RuleViolationDTO{}, // violations are computed at write time
			CreatedAt:      a.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
			UpdatedAt:      a.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	pkg.WriteJSON(w, http.StatusOK, responses)
}

// CreateShift creates a new shift instance.
func (h *ScheduleHandler) CreateShift(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	// Decode canonical DTO: Date time.Time, StartTime/EndTime string (HH:MM), Role *string, Source *string, etc.
	var req dto.CreateShiftInstanceRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Service: CreateShift(ctx, tenantID, req dto.CreateShiftInstanceRequest)
	shift, err := h.svc.CreateShift(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, shiftToResponse(shift))
}

// GetShift returns a specific shift by ID.
func (h *ScheduleHandler) GetShift(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	shiftID, err := uuid.Parse(chi.URLParam(r, "shiftId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid shift ID").WithKey("errors.invalidShiftId"))
		return
	}

	// Service: GetByID(ctx, tenantID, id) — tenantID required
	shift, err := h.svc.GetByID(ctx, storeID, shiftID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, shiftToResponse(shift))
}

// UpdateShift updates an existing shift.
func (h *ScheduleHandler) UpdateShift(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	shiftID, err := uuid.Parse(chi.URLParam(r, "shiftId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid shift ID").WithKey("errors.invalidShiftId"))
		return
	}

	// Decode canonical DTO: all fields optional (*string, *time.Time, *uuid.UUID)
	var req dto.UpdateShiftInstanceRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	// Service: UpdateShift(ctx, tenantID, id, req dto.UpdateShiftInstanceRequest)
	shift, err := h.svc.UpdateShift(ctx, storeID, shiftID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, shiftToResponse(shift))
}

// DeleteShift deletes a shift instance.
func (h *ScheduleHandler) DeleteShift(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	shiftID, err := uuid.Parse(chi.URLParam(r, "shiftId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid shift ID").WithKey("errors.invalidShiftId"))
		return
	}

	// Service: DeleteShift(ctx, tenantID, id)
	if err := h.svc.DeleteShift(ctx, storeID, shiftID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GenerateSchedule projects A/B week templates into ShiftInstances for a date range.
func (h *ScheduleHandler) GenerateSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	// Decode as plain strings — time.Time in JSON requires RFC3339 which is fragile.
	var body struct {
		DateFrom string `json:"date_from"`
		DateTo   string `json:"date_to"`
	}
	if err := pkg.DecodeJSON(r, &body); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}
	if body.DateFrom == "" || body.DateTo == "" {
		pkg.WriteError(w, apierror.BadRequest("date_from and date_to are required (YYYY-MM-DD)").WithKey("errors.invalidInput"))
		return
	}

	dateFrom, err := parseDate(body.DateFrom)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid date_from, use YYYY-MM-DD").WithKey("errors.invalidInput"))
		return
	}
	dateTo, err := parseDate(body.DateTo)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid date_to, use YYYY-MM-DD").WithKey("errors.invalidInput"))
		return
	}

	req := dto.GenerateScheduleRequest{
		StoreID:  storeID,
		DateFrom: dateFrom,
		DateTo:   dateTo,
	}

	count, err := h.svc.ProjectABSchedule(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, map[string]int{"shifts_created": count})
}

// CreateAssignment assigns an employee to a shift.
func (h *ScheduleHandler) CreateAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	// Decode canonical DTO: ShiftInstanceID uuid.UUID, EmployeeID uuid.UUID, Status *string
	var req dto.CreateAssignmentRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	if req.ShiftID == uuid.Nil || req.EmployeeID == uuid.Nil {
		pkg.WriteError(w, apierror.BadRequest("shift_id and employee_id are required").WithKey("errors.invalidInput"))
		return
	}

	// Service: CreateAssignment(ctx, tenantID, req dto.CreateAssignmentRequest)
	// Returns advisory violations (WARNING/INFO) that accompany a successful creation.
	assignment, violations, err := h.svc.CreateAssignment(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	// Translate violations to the DTO shape (includes shift_id / employee_id for
	// the frontend to map violations to calendar cells without extra context).
	violationDTOs := make([]dto.RuleViolationDTO, len(violations))
	for i, v := range violations {
		ruleID, _ := uuid.Parse(v.RuleID) // RuleID is string in model; safe no-op on empty/invalid
		violationDTOs[i] = dto.RuleViolationDTO{
			RuleID:     ruleID,
			RuleType:   v.RuleType,
			Severity:   v.Severity,
			Message:    v.Message,
			Key:        v.Key,
			Params:     v.Params,
			ShiftID:    assignment.ShiftInstanceID,
			EmployeeID: assignment.EmployeeID,
		}
	}

	// Return the same AssignmentResponse shape as ListAssignments so the
	// frontend can substitute it directly into the assignments array.
	pkg.WriteJSON(w, http.StatusCreated, dto.AssignmentResponse{
		ID:             assignment.ID,
		StoreID:        assignment.TenantID,
		ShiftID:        assignment.ShiftInstanceID,
		EmployeeID:     assignment.EmployeeID,
		ShiftDate:      assignment.ShiftDate.Format("2006-01-02"),
		ShiftStartTime: assignment.ShiftStartTime,
		ShiftEndTime:   assignment.ShiftEndTime,
		Violations:     violationDTOs,
		CreatedAt:      assignment.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      assignment.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// GetAssignments returns all assignments for a shift.
func (h *ScheduleHandler) GetAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	shiftID, err := uuid.Parse(chi.URLParam(r, "shiftId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid shift ID").WithKey("errors.invalidShiftId"))
		return
	}

	// Service: GetAssignments(ctx, tenantID, shiftID)
	assignments, err := h.svc.GetAssignments(ctx, storeID, shiftID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]assignmentResponse, len(assignments))
	for i, a := range assignments {
		responses[i] = assignToResponse(a)
	}

	pkg.WriteJSON(w, http.StatusOK, responses)
}

// ListEmployeeSchedule returns the enriched shift list for a specific employee.
// Managers use this to view any employee's upcoming/past shifts.
func (h *ScheduleHandler) ListEmployeeSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	employeeID, err := uuid.Parse(chi.URLParam(r, "employeeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid employee ID").WithKey("errors.invalidEmployeeId"))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	if fromStr == "" || toStr == "" {
		pkg.WriteError(w, apierror.BadRequest("from and to query parameters are required").WithKey("errors.missingParams"))
		return
	}

	from, err := parseDate(fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	to, err := parseDate(toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	// Service: ListMyShifts(ctx, tenantID, employeeID, from, to) → ([]*ShiftWithAssignment, error)
	shifts, err := h.svc.ListMyShifts(ctx, storeID, employeeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	if shifts == nil {
		pkg.WriteJSON(w, http.StatusOK, []*service.ShiftWithAssignment{})
		return
	}

	pkg.WriteJSON(w, http.StatusOK, shifts)
}

// PublishSchedule transitions all shifts in a date range from DRAFT to PUBLISHED.
func (h *ScheduleHandler) PublishSchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	from, err := parseDate(req.From)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	to, err := parseDate(req.To)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD").WithKey("errors.invalidDateRange"))
		return
	}

	// Service: PublishSchedule(ctx, tenantID, from, to) → (int64, error)
	count, err := h.svc.PublishSchedule(ctx, storeID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, map[string]int64{"shifts_published": count})
}

// DeleteAssignment removes a shift assignment by ID.
func (h *ScheduleHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	assignmentID, err := uuid.Parse(chi.URLParam(r, "assignmentId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid assignment ID").WithKey("errors.invalidInput"))
		return
	}

	if err := h.svc.DeleteAssignment(ctx, storeID, assignmentID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ResetWeekAssignments deletes all assignments for the requested week.
// Route: DELETE /stores/{storeId}/assignments?week_of=YYYY-MM-DD
// The week_of date must be a Monday (the start of the week to reset).
func (h *ScheduleHandler) ResetWeekAssignments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	weekOfStr := r.URL.Query().Get("week_of")
	if weekOfStr == "" {
		pkg.WriteError(w, apierror.BadRequest("week_of query parameter is required (YYYY-MM-DD)").WithKey("errors.invalidInput"))
		return
	}

	weekStart, err := time.Parse("2006-01-02", weekOfStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("week_of must be in YYYY-MM-DD format").WithKey("errors.invalidInput"))
		return
	}

	deleted, err := h.svc.ResetWeekAssignments(ctx, storeID, weekStart)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"deleted":    deleted,
		"week_start": weekStart.Format("2006-01-02"),
	})
}

// RegenerateWeek hard-resets a week by deleting all shifts and assignments then
// re-projecting from A/B templates. Accepts ?week_of=YYYY-MM-DD.
func (h *ScheduleHandler) RegenerateWeek(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	weekOfStr := r.URL.Query().Get("week_of")
	if weekOfStr == "" {
		pkg.WriteError(w, apierror.BadRequest("week_of query parameter is required").WithKey("errors.invalidInput"))
		return
	}

	weekStart, err := time.Parse("2006-01-02", weekOfStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("week_of must be in YYYY-MM-DD format").WithKey("errors.invalidInput"))
		return
	}

	count, err := h.svc.RegenerateWeek(ctx, storeID, weekStart)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, map[string]interface{}{
		"generated":  count,
		"week_start": weekStart.Format("2006-01-02"),
	})
}
