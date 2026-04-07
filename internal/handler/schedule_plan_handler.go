package handler

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// SchedulePlanHandler handles schedule plan lifecycle HTTP requests.
type SchedulePlanHandler struct {
	svc *service.SchedulePlanService
}

// NewSchedulePlanHandler creates a new schedule plan handler.
func NewSchedulePlanHandler(svc *service.SchedulePlanService) *SchedulePlanHandler {
	return &SchedulePlanHandler{svc: svc}
}

// GetByID returns a single plan by its ID.
func (h *SchedulePlanHandler) GetByID(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid plan ID"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"op":        "SchedulePlanHandler.GetByID",
	}).Debug("handling get plan by ID request")

	plan, err := h.svc.GetByID(ctx, tenantID, planID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"plan_id": planID,
		"state":   plan.State,
	}).Debug("plan returned by ID")

	pkg.WriteJSON(w, http.StatusOK, h.svc.PlanToResponse(plan))
}

// Get returns or creates a plan for a given week.
func (h *SchedulePlanHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	weekStr := r.URL.Query().Get("week")
	if weekStr == "" {
		pkg.WriteError(w, apierror.BadRequest("week query parameter required (YYYY-MM-DD)"))
		return
	}

	weekStart, err := time.Parse("2006-01-02", weekStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid week format (expected YYYY-MM-DD)"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"week_start": weekStr,
		"op":         "SchedulePlanHandler.Get",
	}).Debug("handling get/create plan request")

	plan, err := h.svc.GetOrCreate(ctx, tenantID, weekStart)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"plan_id": plan.ID,
		"state":   plan.State,
	}).Debug("plan returned")

	pkg.WriteJSON(w, http.StatusOK, h.svc.PlanToResponse(plan))
}

// Publish transitions a plan from DRAFT to PUBLISHED.
func (h *SchedulePlanHandler) Publish(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	userID := ctxutil.GetUserID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid plan ID"))
		return
	}

	var req dto.PublishPlanRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"user_id":   userID,
		"op":        "SchedulePlanHandler.Publish",
	}).Info("handling publish plan request")

	plan, err := h.svc.Publish(ctx, tenantID, planID, userID, req.Note)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"plan_id": planID,
		"state":   plan.State,
	}).Info("plan published successfully")

	pkg.WriteJSON(w, http.StatusOK, h.svc.PlanToResponse(plan))
}

// RecordOverride records a post-publication edit.
func (h *SchedulePlanHandler) RecordOverride(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	userID := ctxutil.GetUserID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid plan ID"))
		return
	}

	var req dto.RecordOverrideRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"shift_id":  req.ShiftID,
		"user_id":   userID,
		"op":        "SchedulePlanHandler.RecordOverride",
	}).Info("handling record override request")

	entry := model.OverrideEntry{
		ShiftID:   req.ShiftID,
		EditorID:  userID,
		Reason:    req.Reason,
		Timestamp: time.Now(),
	}

	if err := h.svc.RecordOverride(ctx, tenantID, planID, entry); err != nil {
		pkg.WriteError(w, err)
		return
	}

	logger.WithField("plan_id", planID).Info("override recorded")

	pkg.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// GetHistory returns all snapshots for a plan.
func (h *SchedulePlanHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid plan ID"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"op":        "SchedulePlanHandler.GetHistory",
	}).Debug("handling get plan history request")

	snapshots, err := h.svc.GetHistory(ctx, tenantID, planID)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	responses := make([]dto.PlanSnapshotResponse, len(snapshots))
	for i, s := range snapshots {
		responses[i] = dto.PlanSnapshotResponse{
			Version:     s.Version,
			CapturedAt:  s.CapturedAt.Format(time.RFC3339),
			ShiftCount:  s.ShiftCount,
			PublishedBy: s.PublishedBy,
		}
	}

	logger.WithFields(logrus.Fields{
		"plan_id":  planID,
		"versions": len(snapshots),
	}).Debug("plan history returned")

	pkg.WriteJSON(w, http.StatusOK, map[string]interface{}{"snapshots": responses})
}

// Rollback reverts a plan from PUBLISHED/LIVE back to DRAFT.
func (h *SchedulePlanHandler) Rollback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	tenantID := ctxutil.GetTenantID(ctx)
	logger := ctxutil.GetLogger(ctx)

	storeID, err := uuid.Parse(chi.URLParam(r, "storeId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid store ID"))
		return
	}

	if !validateStoreTenant(tenantID, storeID, ctxutil.GetUserRole(ctx)) {
		pkg.WriteError(w, apierror.Forbidden("access denied to this store"))
		return
	}

	planID, err := uuid.Parse(chi.URLParam(r, "planId"))
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid plan ID"))
		return
	}

	var req dto.RollbackPlanRequest
	if err := pkg.DecodeJSON(r, &req); err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid request body"))
		return
	}

	logger.WithFields(logrus.Fields{
		"tenant_id":        tenantID,
		"plan_id":          planID,
		"snapshot_version": req.SnapshotVersion,
		"op":               "SchedulePlanHandler.Rollback",
	}).Info("handling rollback plan request")

	plan, err := h.svc.Rollback(ctx, tenantID, planID, req.SnapshotVersion, req.Note)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"plan_id": planID,
		"state":   plan.State,
	}).Info("plan rolled back successfully")

	pkg.WriteJSON(w, http.StatusOK, h.svc.PlanToResponse(plan))
}
