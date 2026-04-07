package handler

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// RuleHandler handles scheduling rule HTTP requests.
type RuleHandler struct {
	svc *service.RuleService
}

// NewRuleHandler creates a new RuleHandler.
func NewRuleHandler(svc *service.RuleService) *RuleHandler {
	return &RuleHandler{svc: svc}
}

// List returns all rules for the store (tenant).
func (h *RuleHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	rules, err := h.svc.List(ctx, storeID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.RuleResponse, len(rules))
	for i, rule := range rules {
		responses[i] = dto.ToRuleResponse(rule)
	}
	pkg.WriteJSON(w, http.StatusOK, responses)
}

// Get returns a single rule by ID.
func (h *RuleHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "ruleId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid rule ID"))
		return
	}

	rule, err := h.svc.GetByID(ctx, storeID, ruleID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, dto.ToRuleResponse(rule))
}

// Create creates a new scheduling rule.
func (h *RuleHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	var req dto.CreateRuleRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	rule, err := h.svc.Create(ctx, storeID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusCreated, dto.ToRuleResponse(rule))
}

// Update modifies an existing rule.
func (h *RuleHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "ruleId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid rule ID"))
		return
	}

	var req dto.UpdateRuleRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	rule, err := h.svc.Update(ctx, storeID, ruleID, req)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}
	pkg.WriteJSON(w, http.StatusOK, dto.ToRuleResponse(rule))
}

// Delete removes a rule.
func (h *RuleHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}
	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	ruleID, err := uuid.Parse(chi.URLParam(r, "ruleId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid rule ID"))
		return
	}

	if err := h.svc.Delete(ctx, storeID, ruleID); err != nil {
		pkg.WriteError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
