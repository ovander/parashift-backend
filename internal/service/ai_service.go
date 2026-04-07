package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/aigateway"
	"github.com/ovander/backendkit/ainarration"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// AIService implements the three AI sub-engines described in the HLS:
// Recommendation (suggest assignments), Optimization (improve schedules), and
// Insight (surface coverage/rest/cost anomalies as persistent AIInsight records).
//
// When the LLM gateway is unavailable or returns an error, SuggestAssignment
// falls back to the deterministic heuristic scorer in scoring_engine.go.
type AIService struct {
	gateway      *aigateway.Client
	narration    *ainarration.NarrationCache
	insightRepo  repo.AIInsightRepository
	empRepo      repo.EmployeeRepository
	shiftRepo    repo.ShiftInstanceRepository
	assignRepo   repo.ShiftAssignmentRepository
	coverageRepo repo.CoverageRequirementRepository
	contractRepo repo.ContractRepository
	logger       *logrus.Entry
}

// NewAIService creates a new AIService.
func NewAIService(
	gateway *aigateway.Client,
	insightRepo repo.AIInsightRepository,
	empRepo repo.EmployeeRepository,
	shiftRepo repo.ShiftInstanceRepository,
	assignRepo repo.ShiftAssignmentRepository,
	coverageRepo repo.CoverageRequirementRepository,
	contractRepo repo.ContractRepository,
	logger *logrus.Entry,
) *AIService {
	nc := ainarration.NewNarrationCache(ainarration.CacheConfig{
		MaxSize: 256,
		TTL:     10 * time.Minute,
	})
	return &AIService{
		gateway:      gateway,
		narration:    nc,
		insightRepo:  insightRepo,
		empRepo:      empRepo,
		shiftRepo:    shiftRepo,
		assignRepo:   assignRepo,
		coverageRepo: coverageRepo,
		contractRepo: contractRepo,
		logger:       logger,
	}
}

// ─── Recommendation Engine ─────────────────────────────────────────────────────

// SuggestAssignment asks the AI to rank employees for a given shift and returns
// an ordered list of suggestions. Results are cached per (tenantID, shiftID).
func (s *AIService) SuggestAssignment(
	ctx context.Context,
	tenantID uuid.UUID,
	req dto.SuggestAssignmentRequest,
) (*dto.SuggestAssignmentResponse, error) {
	logger := ctxutil.GetLogger(ctx)

	cacheKey := ainarration.CacheKey("suggest", "scheduler", map[string]string{
		"tenant": tenantID.String(),
		"shift":  req.ShiftID.String(),
	})
	if cached, ok := s.narration.Get(tenantID, cacheKey); ok {
		var resp dto.SuggestAssignmentResponse
		if err := json.Unmarshal([]byte(cached.Narrative), &resp); err == nil {
			return &resp, nil
		}
	}

	// Load shift
	shift, err := s.shiftRepo.GetByID(ctx, tenantID, req.ShiftID)
	if err != nil || shift == nil {
		return nil, apierror.NotFound("shift", req.ShiftID.String())
	}

	// Load employees
	employees, _, err := s.empRepo.List(ctx, tenantID, 1, 200)
	if err != nil {
		logger.WithError(err).Error("ai: failed to load employees")
		return nil, apierror.Internal("failed to load employees")
	}

	// Try LLM first; fall back to the heuristic scorer on any error or empty result.
	var suggestions []dto.ScheduleSuggestion

	if s.gateway != nil {
		prompt := buildSuggestPrompt(shift, employees)
		raw, llmErr := s.gateway.Call(ctx, prompt)
		if llmErr != nil {
			logger.WithError(llmErr).Warn("ai: LLM suggestion failed — falling back to heuristic scorer")
		} else if err := aigateway.ExtractJSONInto(raw, &suggestions); err != nil {
			logger.WithError(err).Warn("ai: failed to parse LLM suggestion — falling back to heuristic scorer")
			suggestions = nil
		}
	}

	if len(suggestions) == 0 {
		suggestions = s.heuristicSuggest(ctx, tenantID, shift, employees)
	}

	resp := &dto.SuggestAssignmentResponse{
		ShiftID:     req.ShiftID,
		Suggestions: suggestions,
	}

	// Cache the response (LLM or heuristic).
	if b, err := json.Marshal(resp); err == nil {
		s.narration.Put(tenantID, cacheKey, &ainarration.NarrationOutput{Narrative: string(b)})
	}

	return resp, nil
}

// heuristicSuggest ranks the given employee list for the shift using the
// deterministic scoring engine. It loads weekly hours from the DB once per
// employee and delegates to scoreEmployees for the pure ranking step.
func (s *AIService) heuristicSuggest(
	ctx context.Context,
	tenantID uuid.UUID,
	shift *model.ShiftInstance,
	employees []*model.Employee,
) []dto.ScheduleSuggestion {
	weekStart, weekEnd := weekBounds(shift.Date)

	// Build weekly-hours and contract-hours maps for each candidate.
	weeklyHours := make(map[uuid.UUID]float64, len(employees))
	contractHours := make(map[uuid.UUID]float64, len(employees))

	for _, emp := range employees {
		// Weekly hours from existing assignments.
		assignments, err := s.assignRepo.ListByEmployee(ctx, tenantID, emp.ID, weekStart, weekEnd)
		if err == nil {
			for _, a := range assignments {
				weeklyHours[emp.ID] += shiftHours(a.ShiftStartTime, a.ShiftEndTime)
			}
		}
		// Contract target hours.
		if contract, err := s.contractRepo.GetByEmployeeID(ctx, tenantID, emp.ID); err == nil && contract != nil {
			contractHours[emp.ID] = contract.WeeklyHours
		}
	}

	scored := scoreEmployees(shift, employees, weeklyHours, contractHours)

	// Return top 3.
	n := 3
	if len(scored) < n {
		n = len(scored)
	}
	suggestions := make([]dto.ScheduleSuggestion, n)
	for i := 0; i < n; i++ {
		se := scored[i]
		reason := ""
		if len(se.Reasons) > 0 {
			reason = se.Reasons[0]
			for _, r := range se.Reasons[1:] {
				reason += "; " + r
			}
		}
		suggestions[i] = dto.ScheduleSuggestion{
			EmployeeID:   se.Employee.ID,
			EmployeeName: se.Employee.Name,
			Reason:       reason,
			Confidence:   se.Score / 100.0,
		}
	}
	return suggestions
}

// ─── Optimization Engine ───────────────────────────────────────────────────────

// OptimizeSchedule asks the AI to analyse a week's schedule and return
// re-assignment recommendations that improve coverage, rest, and fairness.
func (s *AIService) OptimizeSchedule(
	ctx context.Context,
	tenantID uuid.UUID,
	req dto.OptimizeScheduleRequest,
) (*dto.OptimizeScheduleResponse, error) {
	logger := ctxutil.GetLogger(ctx)

	cacheKey := ainarration.CacheKey("optimize", "scheduler", map[string]string{
		"tenant": tenantID.String(),
		"from":   req.DateFrom.Format("2006-01-02"),
		"to":     req.DateTo.Format("2006-01-02"),
	})
	if cached, ok := s.narration.Get(tenantID, cacheKey); ok {
		var resp dto.OptimizeScheduleResponse
		if err := json.Unmarshal([]byte(cached.Narrative), &resp); err == nil {
			return &resp, nil
		}
	}

	shifts, _, err := s.shiftRepo.ListByDateRange(ctx, tenantID, req.DateFrom, req.DateTo, 1, 500)
	if err != nil {
		logger.WithError(err).Error("ai: failed to load shifts for optimisation")
		return nil, apierror.Internal("failed to load shifts")
	}

	employees, _, err := s.empRepo.List(ctx, tenantID, 1, 200)
	if err != nil {
		logger.WithError(err).Error("ai: failed to load employees for optimisation")
		return nil, apierror.Internal("failed to load employees")
	}

	prompt := buildOptimizePrompt(req.DateFrom, req.DateTo, shifts, employees)
	raw, err := s.gateway.Call(ctx, prompt)
	if err != nil {
		logger.WithError(err).Warn("ai: optimisation call failed, returning empty suggestions")
		return &dto.OptimizeScheduleResponse{DateFrom: req.DateFrom, DateTo: req.DateTo}, nil
	}

	var suggestions []dto.OptimizeScheduleSuggestion
	if err := aigateway.ExtractJSONInto(raw, &suggestions); err != nil {
		logger.WithError(err).Warn("ai: failed to parse optimisation response")
		return &dto.OptimizeScheduleResponse{DateFrom: req.DateFrom, DateTo: req.DateTo}, nil
	}

	resp := &dto.OptimizeScheduleResponse{
		DateFrom:    req.DateFrom,
		DateTo:      req.DateTo,
		Suggestions: suggestions,
	}
	if b, err := json.Marshal(resp); err == nil {
		s.narration.Put(tenantID, cacheKey, &ainarration.NarrationOutput{Narrative: string(b)})
	}
	return resp, nil
}

// ─── Insight Engine ────────────────────────────────────────────────────────────

// GenerateInsights asks the AI to analyse the current schedule and persist any
// newly discovered insights (coverage gaps, rest violations, cost anomalies).
// This is typically called by a background job, not directly by users.
func (s *AIService) GenerateInsights(ctx context.Context, tenantID uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	from := time.Now().Truncate(24 * time.Hour)
	to := from.AddDate(0, 0, 7)

	shifts, _, err := s.shiftRepo.ListByDateRange(ctx, tenantID, from, to, 1, 500)
	if err != nil {
		logger.WithError(err).Error("ai: failed to load shifts for insight generation")
		return apierror.Internal("failed to load shifts")
	}

	employees, _, err := s.empRepo.List(ctx, tenantID, 1, 200)
	if err != nil {
		logger.WithError(err).Error("ai: failed to load employees for insight generation")
		return apierror.Internal("failed to load employees")
	}

	prompt := buildInsightPrompt(from, to, shifts, employees)
	raw, err := s.gateway.Call(ctx, prompt)
	if err != nil {
		logger.WithError(err).Warn("ai: insight generation call failed, skipping")
		return nil // degrade gracefully
	}

	type rawInsight struct {
		Type           string  `json:"type"`
		Message        string  `json:"message"`
		Recommendation string  `json:"recommendation"`
		Confidence     float64 `json:"confidence"`
	}
	var rawInsights []rawInsight
	if err := aigateway.ExtractJSONInto(raw, &rawInsights); err != nil {
		logger.WithError(err).Warn("ai: failed to parse insights response")
		return nil
	}

	for _, ri := range rawInsights {
		insight := &model.AIInsight{
			TenantScoped: model.TenantScoped{
				ID:        uuid.New(),
				TenantID:  tenantID,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
			Type:           ri.Type,
			Message:        ri.Message,
			Recommendation: ri.Recommendation,
			Confidence:     ri.Confidence,
			Dismissed:      false,
		}
		if err := s.insightRepo.Create(ctx, insight); err != nil {
			logger.WithError(err).Warn("ai: failed to persist insight")
		}
	}
	return nil
}

// ListInsights returns active (non-dismissed) insights for a tenant.
func (s *AIService) ListInsights(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.AIInsight, int64, error) {
	logger := ctxutil.GetLogger(ctx)
	insights, total, err := s.insightRepo.List(ctx, tenantID, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("ai: failed to list insights")
		return nil, 0, apierror.Internal("failed to list insights")
	}
	return insights, total, nil
}

// DismissInsight marks an insight as dismissed.
func (s *AIService) DismissInsight(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)
	if err := s.insightRepo.Dismiss(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("ai: failed to dismiss insight")
		return apierror.Internal("failed to dismiss insight")
	}
	return nil
}

// ─── Prompt builders ───────────────────────────────────────────────────────────

func buildSuggestPrompt(shift *model.ShiftInstance, employees []*model.Employee) string {
	empList := ""
	for _, e := range employees {
		empList += fmt.Sprintf("  - id:%s name:%q job_role:%q\n", e.ID, e.Name, e.JobRole)
	}
	return fmt.Sprintf(`You are a workforce scheduling assistant.
Given the shift details below and the list of available employees, return a JSON array of the top 3 employee suggestions ranked by suitability.

Shift:
  date: %s
  start: %s
  end: %s
  required_qualification: %q

Employees:
%s
Return ONLY valid JSON in this exact format (no explanation):
[{"employee_id":"<uuid>","employee_name":"<name>","reason":"<short reason>","confidence":<0.0-1.0>}]`,
		shift.Date.Format("2006-01-02"), shift.StartTime, shift.EndTime,
		shift.RequiredQualification, empList,
	)
}

func buildOptimizePrompt(from, to time.Time, shifts []*model.ShiftInstance, employees []*model.Employee) string {
	return fmt.Sprintf(`You are a workforce scheduling optimisation assistant.
Analyse the schedule from %s to %s and suggest reassignments to improve coverage, ensure adequate rest, and distribute hours fairly.
There are %d shifts and %d employees.
Return ONLY valid JSON as an array:
[{"shift_id":"<uuid>","recommended_employee_id":"<uuid>","reason":"<short reason>","confidence":<0.0-1.0>}]`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
		len(shifts), len(employees),
	)
}

func buildInsightPrompt(from, to time.Time, shifts []*model.ShiftInstance, employees []*model.Employee) string {
	return fmt.Sprintf(`You are a workforce scheduling analyst.
Analyse the schedule from %s to %s (%d shifts, %d employees) and surface any coverage gaps, rest violations, or fairness issues.
Return ONLY valid JSON as an array:
[{"type":"COVERAGE|REST|FAIRNESS|COST","message":"<observation>","recommendation":"<action>","confidence":<0.0-1.0>}]`,
		from.Format("2006-01-02"), to.Format("2006-01-02"),
		len(shifts), len(employees),
	)
}
