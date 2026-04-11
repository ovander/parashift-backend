package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// SchedulePlanService handles schedule plan lifecycle operations.
type SchedulePlanService struct {
	planRepo  repo.SchedulePlanRepository
	shiftRepo repo.ShiftInstanceRepository
	metricSvc *PlanningModelMetricService
	logger    *logrus.Entry
}

// NewSchedulePlanService creates a new schedule plan service.
func NewSchedulePlanService(planRepo repo.SchedulePlanRepository, shiftRepo repo.ShiftInstanceRepository, logger *logrus.Entry) *SchedulePlanService {
	return &SchedulePlanService{
		planRepo:  planRepo,
		shiftRepo: shiftRepo,
		logger:    logger,
	}
}

// WithMetricService sets the optional metric service dependency.
func (s *SchedulePlanService) WithMetricService(metricSvc *PlanningModelMetricService) *SchedulePlanService {
	s.metricSvc = metricSvc
	return s
}

// GetOrCreate returns an existing plan or creates a new DRAFT plan.
func (s *SchedulePlanService) GetOrCreate(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) (*model.SchedulePlan, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"week_start": weekStart.Format("2006-01-02"),
		"op":         "SchedulePlanService.GetOrCreate",
	}).Debug("fetching or creating schedule plan")

	// Try to find existing plan
	existing, err := s.planRepo.GetByWeekStart(ctx, tenantID, weekStart)
	if err != nil {
		logger.WithError(err).Error("failed to check for existing plan")
		return nil, apierror.Internal("failed to check for existing plan").WithKey("errors.unknown")
	}
	if existing != nil {
		logger.WithField("plan_id", existing.ID).Debug("found existing plan")
		return existing, nil
	}

	logger.Debug("no existing plan found; creating new DRAFT plan")

	// Create new DRAFT plan (storeID == tenantID in single-store tenant model)
	plan := &model.SchedulePlan{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		StoreID:     tenantID, // StoreID == TenantID in single-store model
		WeekStart:   weekStart,
		State:       model.PlanStateDraft,
		Snapshots:   "[]",
		OverrideLog: "[]",
	}

	if err := s.planRepo.Create(ctx, plan); err != nil {
		logger.WithError(err).Error("failed to create new plan")
		return nil, apierror.Internal("failed to create plan").WithKey("errors.unknown")
	}

	logger.WithField("plan_id", plan.ID).Info("schedule plan created (DRAFT)")
	return plan, nil
}

// GetByID retrieves a plan by ID.
func (s *SchedulePlanService) GetByID(ctx context.Context, tenantID, planID uuid.UUID) (*model.SchedulePlan, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"op":        "SchedulePlanService.GetByID",
	}).Debug("fetching plan by ID")

	plan, err := s.planRepo.GetByID(ctx, tenantID, planID)
	if err != nil {
		logger.WithError(err).Error("failed to get plan")
		return nil, apierror.Internal("failed to get plan").WithKey("errors.unknown")
	}
	if plan == nil {
		logger.WithField("plan_id", planID).Warn("plan not found")
		return nil, apierror.NotFound("plan", planID.String()).WithKey("errors.unknown")
	}
	return plan, nil
}

// Publish transitions a plan from DRAFT to PUBLISHED and publishes shifts.
func (s *SchedulePlanService) Publish(ctx context.Context, tenantID, planID uuid.UUID, publishedBy uuid.UUID, note string) (*model.SchedulePlan, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":    tenantID,
		"plan_id":      planID,
		"published_by": publishedBy,
		"op":           "SchedulePlanService.Publish",
	}).Info("publishing schedule plan")

	plan, err := s.planRepo.GetByID(ctx, tenantID, planID)
	if err != nil {
		logger.WithError(err).Error("failed to get plan")
		return nil, apierror.Internal("failed to get plan").WithKey("errors.unknown")
	}
	if plan == nil {
		logger.WithField("plan_id", planID).Warn("plan not found for publish")
		return nil, apierror.NotFound("plan", planID.String()).WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"plan_id":      planID,
		"current_state": plan.State,
	}).Debug("plan state before publish transition")

	if plan.State != model.PlanStateDraft {
		logger.WithField("current_state", plan.State).Warn("publish rejected: plan not in DRAFT state")
		return nil, apierror.ValidationError("plan_already_published", "plan must be in DRAFT state to publish").WithKey("errors.conflict")
	}

	// Get all shifts for this week to count them and calculate coverage
	now := time.Now()
	shifts, _, err := s.shiftRepo.ListByDateRange(ctx, tenantID, plan.WeekStart, plan.WeekStart.AddDate(0, 0, 6), 1, 10000)
	if err != nil {
		logger.WithError(err).Error("failed to list shifts for publish")
		return nil, apierror.Internal("failed to list shifts").WithKey("errors.unknown")
	}

	logger.WithField("shift_count", len(shifts)).Debug("shifts fetched for publish snapshot")

	// Publish all shifts for this week
	if _, err := s.shiftRepo.SetStatusByDateRange(ctx, tenantID, plan.WeekStart, plan.WeekStart.AddDate(0, 0, 6), model.ShiftStatusPublished); err != nil {
		logger.WithError(err).Error("failed to publish shifts")
		return nil, apierror.Internal("failed to publish shifts").WithKey("errors.unknown")
	}

	// Create snapshot
	snapshot := model.PlanSnapshot{
		Version:     1,
		CapturedAt:  now,
		ShiftCount:  len(shifts),
		PublishedBy: publishedBy,
	}

	// Append to snapshots JSON
	var snapshots []model.PlanSnapshot
	if err := json.Unmarshal([]byte(plan.Snapshots), &snapshots); err != nil {
		snapshots = []model.PlanSnapshot{}
	}
	for i := range snapshots {
		snapshots[i].Version = i + 1
	}
	snapshot.Version = len(snapshots) + 1
	snapshots = append(snapshots, snapshot)

	snapshotJSON, _ := json.Marshal(snapshots)

	// Update plan state
	plan.State = model.PlanStatePublished
	plan.PublishedAt = &now
	plan.PublishedBy = &publishedBy
	plan.Snapshots = string(snapshotJSON)
	plan.UpdatedAt = now

	if err := s.planRepo.Update(ctx, plan); err != nil {
		logger.WithError(err).Error("failed to update plan after publish")
		return nil, apierror.Internal("failed to update plan").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"plan_id":   planID,
		"new_state": model.PlanStatePublished,
		"note":      note,
	}).Info("schedule plan published successfully")

	// Record metric if service is available
	if s.metricSvc != nil {
		coverageRate := 1.0 // simplified: assume full coverage
		scheme := plan.ModelScheme
		if scheme == "" {
			scheme = "manual"
		}
		logger.WithField("scheme", scheme).Debug("recording publish metric")
		_ = s.metricSvc.RecordPublish(ctx, tenantID, scheme, plan.WeekStart, coverageRate, 0, 0)
	}

	return plan, nil
}

// RecordOverride records a post-publication edit for audit.
func (s *SchedulePlanService) RecordOverride(ctx context.Context, tenantID, planID uuid.UUID, entry model.OverrideEntry) error {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"shift_id":  entry.ShiftID,
		"editor_id": entry.EditorID,
		"op":        "SchedulePlanService.RecordOverride",
	}).Info("recording plan override")

	plan, err := s.planRepo.GetByID(ctx, tenantID, planID)
	if err != nil {
		logger.WithError(err).Error("failed to get plan")
		return apierror.Internal("failed to get plan").WithKey("errors.unknown")
	}
	if plan == nil {
		logger.WithField("plan_id", planID).Warn("plan not found for override")
		return apierror.NotFound("plan", planID.String()).WithKey("errors.unknown")
	}

	// Only PUBLISHED or LIVE states allow overrides
	if plan.State != model.PlanStatePublished && plan.State != model.PlanStateLive {
		logger.WithField("current_state", plan.State).Warn("override rejected: invalid plan state")
		return apierror.ValidationError("invalid_state", "overrides only allowed in PUBLISHED or LIVE state").WithKey("errors.conflict")
	}

	entry.Timestamp = time.Now()

	// Append to override log
	var overrides []model.OverrideEntry
	if err := json.Unmarshal([]byte(plan.OverrideLog), &overrides); err != nil {
		overrides = []model.OverrideEntry{}
	}
	overrides = append(overrides, entry)

	overrideJSON, _ := json.Marshal(overrides)

	plan.OverrideLog = string(overrideJSON)
	plan.UpdatedAt = time.Now()

	if err := s.planRepo.Update(ctx, plan); err != nil {
		logger.WithError(err).Error("failed to update plan override log")
		return apierror.Internal("failed to update plan").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"plan_id":        planID,
		"override_count": len(overrides),
	}).Info("plan override recorded")

	// Increment adjustment count in metrics
	if s.metricSvc != nil {
		scheme := plan.ModelScheme
		if scheme == "" {
			scheme = "manual"
		}
		logger.WithField("scheme", scheme).Debug("incrementing adjustment metric")
		_ = s.metricSvc.IncrementAdjustment(ctx, tenantID, scheme, plan.WeekStart)
	}

	return nil
}

// GetHistory returns all snapshots for a plan.
func (s *SchedulePlanService) GetHistory(ctx context.Context, tenantID, planID uuid.UUID) ([]model.PlanSnapshot, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"plan_id":   planID,
		"op":        "SchedulePlanService.GetHistory",
	}).Debug("fetching plan history")

	plan, err := s.planRepo.GetByID(ctx, tenantID, planID)
	if err != nil {
		logger.WithError(err).Error("failed to get plan")
		return nil, apierror.Internal("failed to get plan").WithKey("errors.unknown")
	}
	if plan == nil {
		logger.WithField("plan_id", planID).Warn("plan not found for history")
		return nil, apierror.NotFound("plan", planID.String()).WithKey("errors.unknown")
	}

	var snapshots []model.PlanSnapshot
	if err := json.Unmarshal([]byte(plan.Snapshots), &snapshots); err != nil {
		logger.WithError(err).Warn("failed to unmarshal snapshots; returning empty")
		snapshots = []model.PlanSnapshot{}
	}
	logger.WithFields(logrus.Fields{
		"plan_id":        planID,
		"snapshot_count": len(snapshots),
	}).Debug("plan history fetched")
	return snapshots, nil
}

// Rollback reverts a plan from PUBLISHED/LIVE back to DRAFT.
func (s *SchedulePlanService) Rollback(ctx context.Context, tenantID, planID uuid.UUID, snapshotVersion int, note string) (*model.SchedulePlan, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":        tenantID,
		"plan_id":          planID,
		"snapshot_version": snapshotVersion,
		"op":               "SchedulePlanService.Rollback",
	}).Info("rolling back schedule plan to DRAFT")

	plan, err := s.planRepo.GetByID(ctx, tenantID, planID)
	if err != nil {
		logger.WithError(err).Error("failed to get plan")
		return nil, apierror.Internal("failed to get plan").WithKey("errors.unknown")
	}
	if plan == nil {
		logger.WithField("plan_id", planID).Warn("plan not found for rollback")
		return nil, apierror.NotFound("plan", planID.String()).WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"plan_id":       planID,
		"current_state": plan.State,
	}).Debug("plan state before rollback")

	if plan.State == model.PlanStateArchived {
		logger.WithField("plan_id", planID).Warn("rollback rejected: plan is archived")
		return nil, apierror.ValidationError("archived", "cannot rollback archived plan").WithKey("errors.conflict")
	}

	// Revert shifts back to DRAFT
	if _, err := s.shiftRepo.SetStatusByDateRange(ctx, tenantID, plan.WeekStart, plan.WeekStart.AddDate(0, 0, 6), model.ShiftStatusDraft); err != nil {
		logger.WithError(err).Error("failed to revert shifts to draft")
		return nil, apierror.Internal("failed to revert shifts").WithKey("errors.unknown")
	}

	// Revert to DRAFT
	plan.State = model.PlanStateDraft
	plan.UpdatedAt = time.Now()

	if err := s.planRepo.Update(ctx, plan); err != nil {
		logger.WithError(err).Error("failed to update plan after rollback")
		return nil, apierror.Internal("failed to update plan").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"plan_id":   planID,
		"new_state": model.PlanStateDraft,
		"note":      note,
	}).Info("schedule plan rolled back to DRAFT")

	return plan, nil
}

// AdvanceState is called by a background job to transition PUBLISHED → LIVE → ARCHIVED.
func (s *SchedulePlanService) AdvanceState(ctx context.Context, tenantID uuid.UUID, weekStart time.Time) error {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"week_start": weekStart.Format("2006-01-02"),
		"op":         "SchedulePlanService.AdvanceState",
	}).Debug("advancing plan state")

	plan, err := s.planRepo.GetByWeekStart(ctx, tenantID, weekStart)
	if err != nil {
		logger.WithError(err).Error("failed to get plan for state advance")
		return err
	}
	if plan == nil {
		logger.Debug("no plan found for state advance; skipping")
		return nil
	}

	now := time.Now()
	weekEnd := weekStart.AddDate(0, 0, 6)

	// PUBLISHED and weekStart <= today → LIVE
	if plan.State == model.PlanStatePublished && !weekStart.After(now) {
		logger.WithFields(logrus.Fields{
			"plan_id":   plan.ID,
			"old_state": model.PlanStatePublished,
			"new_state": model.PlanStateLive,
		}).Info("plan state advancing: PUBLISHED → LIVE")
		plan.State = model.PlanStateLive
		plan.UpdatedAt = now
		return s.planRepo.Update(ctx, plan)
	}

	// LIVE and weekStart+7 <= today → ARCHIVED
	if plan.State == model.PlanStateLive && !weekEnd.After(now) {
		logger.WithFields(logrus.Fields{
			"plan_id":   plan.ID,
			"old_state": model.PlanStateLive,
			"new_state": model.PlanStateArchived,
		}).Info("plan state advancing: LIVE → ARCHIVED")
		plan.State = model.PlanStateArchived
		plan.UpdatedAt = now
		return s.planRepo.Update(ctx, plan)
	}

	logger.WithFields(logrus.Fields{
		"plan_id":       plan.ID,
		"current_state": plan.State,
	}).Debug("plan state unchanged; no transition required")
	return nil
}

// PlanToResponse converts a SchedulePlan model to its DTO response.
func (s *SchedulePlanService) PlanToResponse(p *model.SchedulePlan) dto.SchedulePlanResponse {
	var snapshots []model.PlanSnapshot
	json.Unmarshal([]byte(p.Snapshots), &snapshots)

	var publishedAt *string
	if p.PublishedAt != nil {
		published := p.PublishedAt.Format(time.RFC3339)
		publishedAt = &published
	}

	return dto.SchedulePlanResponse{
		ID:          p.ID,
		StoreID:     p.StoreID,
		WeekStart:   p.WeekStart.Format("2006-01-02"),
		State:       p.State,
		PublishedAt: publishedAt,
		PublishedBy: p.PublishedBy,
		Version:     len(snapshots),
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}
