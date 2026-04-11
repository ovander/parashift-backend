package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// PlanningModelMetricService handles planning model metrics.
type PlanningModelMetricService struct {
	metricRepo repo.PlanningModelMetricRepository
	logger     *logrus.Entry
}

// NewPlanningModelMetricService creates a new planning model metric service.
func NewPlanningModelMetricService(metricRepo repo.PlanningModelMetricRepository, logger *logrus.Entry) *PlanningModelMetricService {
	return &PlanningModelMetricService{
		metricRepo: metricRepo,
		logger:     logger,
	}
}

// RecordPublish records metrics when a plan is published.
func (s *PlanningModelMetricService) RecordPublish(ctx context.Context, tenantID uuid.UUID, scheme string, weekStart time.Time, coverageRate float64, overtimeHours float64, violationCount int) error {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":       tenantID,
		"scheme":          scheme,
		"week_start":      weekStart.Format("2006-01-02"),
		"coverage_rate":   coverageRate,
		"overtime_hours":  overtimeHours,
		"violation_count": violationCount,
		"op":              "PlanningModelMetricService.RecordPublish",
	}).Info("recording publish metric")

	metric := &model.PlanningModelMetric{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		StoreID:        tenantID, // StoreID == TenantID
		ModelScheme:    scheme,
		WeekStart:      weekStart,
		CoverageRate:   coverageRate,
		OvertimeHours:  overtimeHours,
		AdjustmentCount: 0,
		ViolationCount: violationCount,
	}

	if err := s.metricRepo.Upsert(ctx, metric); err != nil {
		logger.WithError(err).Error("failed to upsert publish metric")
		return apierror.Internal("failed to record metric").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"scheme":     scheme,
		"week_start": weekStart.Format("2006-01-02"),
	}).Debug("publish metric recorded")

	return nil
}

// IncrementAdjustment increments the adjustment count for a published week.
func (s *PlanningModelMetricService) IncrementAdjustment(ctx context.Context, tenantID uuid.UUID, scheme string, weekStart time.Time) error {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":  tenantID,
		"scheme":     scheme,
		"week_start": weekStart.Format("2006-01-02"),
		"op":         "PlanningModelMetricService.IncrementAdjustment",
	}).Debug("incrementing adjustment count")

	// Fetch the metric if it exists
	metrics, err := s.metricRepo.ListByScheme(ctx, tenantID, scheme, 1000)
	if err != nil {
		logger.WithError(err).Error("failed to list metrics for adjustment increment")
		return apierror.Internal("failed to update metric").WithKey("errors.unknown")
	}

	var metric *model.PlanningModelMetric
	for _, m := range metrics {
		if m.WeekStart == weekStart {
			metric = m
			break
		}
	}

	if metric == nil {
		// Create a new metric if none exists
		metric = &model.PlanningModelMetric{
			TenantScoped: model.TenantScoped{
				ID:        uuid.New(),
				TenantID:  tenantID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			StoreID:         tenantID,
			ModelScheme:     scheme,
			WeekStart:       weekStart,
			CoverageRate:    1.0,
			AdjustmentCount: 1,
		}
	} else {
		metric.AdjustmentCount++
		metric.UpdatedAt = time.Now()
	}

	if err := s.metricRepo.Upsert(ctx, metric); err != nil {
		logger.WithError(err).Error("failed to upsert adjustment metric")
		return apierror.Internal("failed to update metric").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"scheme":           scheme,
		"week_start":       weekStart.Format("2006-01-02"),
		"adjustment_count": metric.AdjustmentCount,
	}).Debug("adjustment count incremented")

	return nil
}

// GetMetricsForScheme retrieves and summarizes metrics for a model scheme.
func (s *PlanningModelMetricService) GetMetricsForScheme(ctx context.Context, tenantID uuid.UUID, scheme string, limitWeeks int) (*dto.PlanningModelMetricsResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":   tenantID,
		"scheme":      scheme,
		"limit_weeks": limitWeeks,
		"op":          "PlanningModelMetricService.GetMetricsForScheme",
	}).Debug("fetching metrics for scheme")

	metrics, err := s.metricRepo.ListByScheme(ctx, tenantID, scheme, limitWeeks)
	if err != nil {
		logger.WithError(err).Error("failed to list metrics")
		return nil, apierror.Internal("failed to list metrics").WithKey("errors.unknown")
	}

	// Convert to DTOs
	responses := make([]dto.PlanningModelMetricResponse, len(metrics))
	for i, m := range metrics {
		responses[i] = dto.PlanningModelMetricResponse{
			Scheme:          m.ModelScheme,
			WeekStart:       m.WeekStart.Format("2006-01-02"),
			CoverageRate:    m.CoverageRate,
			OvertimeHours:   m.OvertimeHours,
			AdjustmentCount: m.AdjustmentCount,
			ViolationCount:  m.ViolationCount,
		}
	}

	// Calculate summary
	summary := dto.PlanningModelMetricSummary{
		SampleWeeks: len(metrics),
	}

	if len(metrics) > 0 {
		totalCoverage := 0.0
		totalOvertime := 0.0
		totalAdjustments := 0.0

		for _, m := range metrics {
			totalCoverage += m.CoverageRate
			totalOvertime += m.OvertimeHours
			totalAdjustments += float64(m.AdjustmentCount)
		}

		summary.AvgCoverageRate = totalCoverage / float64(len(metrics))
		summary.AvgOvertimeHours = totalOvertime / float64(len(metrics))
		summary.AvgAdjustmentCount = totalAdjustments / float64(len(metrics))
	}

	logger.WithFields(logrus.Fields{
		"scheme":       scheme,
		"sample_weeks": summary.SampleWeeks,
		"avg_coverage": summary.AvgCoverageRate,
	}).Debug("metrics summary computed")

	return &dto.PlanningModelMetricsResponse{
		Scheme:  scheme,
		Metrics: responses,
		Summary: summary,
	}, nil
}
