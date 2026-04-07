package service

import (
	"fmt"
	"sort"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
)

// ScoredEmployee is the result of scoring a candidate employee for a shift.
type ScoredEmployee struct {
	Employee *model.Employee
	Score    float64  // 0–100
	Reasons  []string // human-readable explanation of each contributing factor
}

// scoreEmployees ranks employees for the given shift using a deterministic
// heuristic. It is a pure function: no I/O, no side effects.
//
// Scoring breakdown (100 pts total):
//   - Role match   (50 pts) — employee role matches shift's required qualification
//   - Hours balance (50 pts) — inversely proportional to hours already worked this
//     week relative to contractWeeklyHours (or 40 h default when unknown)
//
// weeklyHours maps employeeID → hours already worked in the current week.
// contractHours maps employeeID → contracted weekly target (0 = unknown/use default).
//
// Returns employees sorted by descending score. Employees with score 0 (role
// mismatch when a role is required) are excluded from the result.
func scoreEmployees(
	shift *model.ShiftInstance,
	employees []*model.Employee,
	weeklyHours map[uuid.UUID]float64,
	contractHours map[uuid.UUID]float64,
) []ScoredEmployee {
	const defaultContractHours = 40.0

	results := make([]ScoredEmployee, 0, len(employees))

	for _, emp := range employees {
		var score float64
		var reasons []string

		// ── Role match (50 pts) ─────────────────────────────────────────────────
		required := shift.RequiredQualification
		if required == "" {
			required = shift.Role
		}
		if required != "" {
			if emp.JobRole == required {
				score += 50
				reasons = append(reasons, fmt.Sprintf("job role matches (%s)", required))
			} else {
				// Required job role not met — skip entirely.
				continue
			}
		} else {
			// No role requirement — all employees get the points.
			score += 50
			reasons = append(reasons, "no role restriction")
		}

		// ── Hours balance (50 pts) ──────────────────────────────────────────────
		target := contractHours[emp.ID]
		if target <= 0 {
			target = defaultContractHours
		}
		worked := weeklyHours[emp.ID] // 0 if not in map

		// Proposed shift hours will be added on top.
		proposed := shiftHours(shift.StartTime, shift.EndTime)
		projectedTotal := worked + proposed

		var balanceScore float64
		if projectedTotal <= target {
			// Under or exactly at target — linear scale from 50 pts (0 h) to 0 pts (target h).
			balanceScore = 50 * (1 - worked/target)
			reasons = append(reasons, fmt.Sprintf("%.1f h worked (target %.0f h)", worked, target))
		} else {
			// Over target — still score but penalised; 0 pts at 2× the target.
			excess := projectedTotal - target
			balanceScore = 50 * (1 - excess/target)
			if balanceScore < 0 {
				balanceScore = 0
			}
			reasons = append(reasons, fmt.Sprintf("%.1f h worked, over target (%.0f h)", worked, target))
		}
		score += balanceScore

		results = append(results, ScoredEmployee{
			Employee: emp,
			Score:    score,
			Reasons:  reasons,
		})
	}

	// Sort descending by score (stable so ordering is deterministic among ties).
	sort.SliceStable(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}
