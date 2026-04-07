package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// PlanningModelMetricHandler handles planning model metric HTTP requests.
type PlanningModelMetricHandler struct {
	svc *service.PlanningModelMetricService
}

// NewPlanningModelMetricHandler creates a new planning model metric handler.
func NewPlanningModelMetricHandler(svc *service.PlanningModelMetricService) *PlanningModelMetricHandler {
	return &PlanningModelMetricHandler{svc: svc}
}

// GetMetrics returns metrics for a planning model scheme.
func (h *PlanningModelMetricHandler) GetMetrics(w http.ResponseWriter, r *http.Request) {
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

	scheme := chi.URLParam(r, "scheme")
	if scheme == "" {
		pkg.WriteError(w, apierror.BadRequest("scheme parameter required"))
		return
	}

	weeksStr := r.URL.Query().Get("weeks")
	weeks := 8 // default
	if weeksStr != "" {
		if w, err := strconv.Atoi(weeksStr); err == nil && w > 0 {
			weeks = w
		}
	}

	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"scheme":    scheme,
		"weeks":     weeks,
		"op":        "PlanningModelMetricHandler.GetMetrics",
	}).Debug("handling get planning model metrics request")

	metrics, err := h.svc.GetMetricsForScheme(ctx, tenantID, scheme, weeks)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	logger.WithFields(logrus.Fields{
		"scheme":       scheme,
		"sample_weeks": metrics.Summary.SampleWeeks,
	}).Debug("metrics returned")

	pkg.WriteJSON(w, http.StatusOK, metrics)
}
