package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// QualificationHandler handles qualification HTTP requests.
type QualificationHandler struct {
	svc *service.QualificationService
}

// NewQualificationHandler creates a new qualification handler.
func NewQualificationHandler(svc *service.QualificationService) *QualificationHandler {
	return &QualificationHandler{svc: svc}
}

// ─── Store-level qualification CRUD ──────────────────────────────────────────

// List returns all qualifications for a store.
func (h *QualificationHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"op":        "QualificationHandler.List",
	}).Debug("handling list qualifications request")

	quals, err := h.svc.ListQualifications(ctx, tenantID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, dto.QualificationListResponse{Qualifications: quals})
}

// Create creates a new qualification.
func (h *QualificationHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	var req dto.CreateQualificationRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"name":      req.Name,
		"op":        "QualificationHandler.Create",
	}).Info("handling create qualification request")

	qual, err := h.svc.CreateQualification(ctx, tenantID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, qual)
}

// Update updates an existing qualification.
func (h *QualificationHandler) Update(w http.ResponseWriter, r *http.Request) {
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

	qualID, err := uuid.Parse(chi.URLParam(r, "qualId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid qualification ID").WithKey("errors.invalidInput"))
		return
	}

	var req dto.UpdateQualificationRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	qual, err := h.svc.UpdateQualification(ctx, tenantID, qualID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, qual)
}

// Delete removes a qualification.
func (h *QualificationHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID").WithKey("errors.invalidStoreId"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store").WithKey("errors.accessDenied"))
		return
	}

	qualID, err := uuid.Parse(chi.URLParam(r, "qualId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid qualification ID").WithKey("errors.invalidInput"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"qual_id":   qualID,
		"op":        "QualificationHandler.Delete",
	}).Info("handling delete qualification request")

	if err := h.svc.DeleteQualification(ctx, tenantID, qualID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusNoContent, nil)
}

// ListExpiring returns qualifications expiring within N days.
func (h *QualificationHandler) ListExpiring(w http.ResponseWriter, r *http.Request) {
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

	daysStr := r.URL.Query().Get("days")
	days := 30 // default
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil {
			days = d
		}
	}

	expiring, err := h.svc.ListExpiringQualifications(ctx, tenantID, days)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, dto.EmployeeQualificationListResponse{Qualifications: expiring})
}

// ─── Employee-level qualification CRUD ───────────────────────────────────────

// ListForEmployee returns all qualifications held by an employee.
func (h *QualificationHandler) ListForEmployee(w http.ResponseWriter, r *http.Request) {
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

	if !enforceSelfOrManager(w, r, employeeID) {
		return
	}

	quals, err := h.svc.ListEmployeeQualifications(ctx, tenantID, employeeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, dto.EmployeeQualificationListResponse{Qualifications: quals})
}

// AddToEmployee assigns a qualification to an employee.
func (h *QualificationHandler) AddToEmployee(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

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

	var req dto.AddEmployeeQualificationRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id":        tenantID,
		"employee_id":      employeeID,
		"qualification_id": req.QualificationID,
		"op":               "QualificationHandler.AddToEmployee",
	}).Info("handling add qualification to employee request")

	qual, err := h.svc.AddEmployeeQualification(ctx, tenantID, employeeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusCreated, qual)
}

// UpdateForEmployee updates an employee's qualification record.
func (h *QualificationHandler) UpdateForEmployee(w http.ResponseWriter, r *http.Request) {
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

	eqID, err := uuid.Parse(chi.URLParam(r, "eqId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid qualification ID").WithKey("errors.invalidInput"))
		return
	}

	var req dto.UpdateEmployeeQualificationRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body").WithKey("errors.invalidInput"))
		return
	}

	qual, err := h.svc.UpdateEmployeeQualification(ctx, tenantID, eqID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusOK, qual)
}

// RemoveFromEmployee removes a qualification from an employee.
func (h *QualificationHandler) RemoveFromEmployee(w http.ResponseWriter, r *http.Request) {
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

	eqID, err := uuid.Parse(chi.URLParam(r, "eqId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid qualification ID").WithKey("errors.invalidInput"))
		return
	}

	if err := h.svc.RemoveEmployeeQualification(ctx, tenantID, eqID); err != nil {
		pkg.WriteError(w, err)
		return
	}

	pkg.WriteJSON(w, http.StatusNoContent, nil)
}
