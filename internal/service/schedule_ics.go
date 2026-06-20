package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
)

// icsExporter renders an employee's assigned shifts as an iCalendar (RFC 5545)
// document. Extracted from ScheduleService (ARC-1): data loading lives in
// Generate, while the pure formatting in renderICS is unit-testable without a DB.
type icsExporter struct {
	empRepo    repo.EmployeeRepository
	assignRepo repo.ShiftAssignmentRepository
	shiftRepo  repo.ShiftInstanceRepository
}

func newICSExporter(empRepo repo.EmployeeRepository, assignRepo repo.ShiftAssignmentRepository, shiftRepo repo.ShiftInstanceRepository) *icsExporter {
	return &icsExporter{empRepo: empRepo, assignRepo: assignRepo, shiftRepo: shiftRepo}
}

// Generate loads the employee's assignments and their shifts for [from, to] and
// renders them as an ICS calendar.
func (e *icsExporter) Generate(ctx context.Context, tenantID, employeeID uuid.UUID, from, to time.Time) (string, error) {
	logger := ctxutil.GetLogger(ctx)

	emp, err := e.empRepo.GetByID(ctx, tenantID, employeeID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return "", apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return "", apierror.NotFound("employee", employeeID.String()).WithKey("errors.unknown")
	}

	assignments, err := e.assignRepo.ListByEmployee(ctx, tenantID, employeeID, from, to)
	if err != nil {
		logger.WithError(err).Error("failed to get assignments")
		return "", apierror.Internal("failed to get assignments").WithKey("errors.unknown")
	}

	// Batch-load all required shifts in one query (avoids N+1).
	var shiftByID map[uuid.UUID]*model.ShiftInstance
	if len(assignments) > 0 {
		ids := make([]uuid.UUID, 0, len(assignments))
		seen := make(map[uuid.UUID]bool, len(assignments))
		for _, a := range assignments {
			if !seen[a.ShiftInstanceID] {
				ids = append(ids, a.ShiftInstanceID)
				seen[a.ShiftInstanceID] = true
			}
		}
		batchShifts, err := e.shiftRepo.ListByIDs(ctx, tenantID, ids)
		if err != nil {
			logger.WithError(err).Error("failed to batch-load shifts for ICS")
			return "", apierror.Internal("failed to load shifts").WithKey("errors.unknown")
		}
		shiftByID = make(map[uuid.UUID]*model.ShiftInstance, len(batchShifts))
		for _, sh := range batchShifts {
			shiftByID[sh.ID] = sh
		}
	}

	return renderICS(assignments, shiftByID), nil
}

// renderICS builds an iCalendar document from assignments and their shifts. It is
// pure (no I/O), so it can be unit-tested in isolation. Assignments whose shift
// is missing from shiftByID are skipped.
func renderICS(assignments []*model.ShiftAssignment, shiftByID map[uuid.UUID]*model.ShiftInstance) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//ParaShift//WFM//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")

	for _, assignment := range assignments {
		shift, ok := shiftByID[assignment.ShiftInstanceID]
		if !ok {
			continue
		}

		// iCalendar date-time format: YYYYMMDDTHHMMSS
		dateStr := shift.Date.Format("20060102")
		startTimeStr := strings.ReplaceAll(shift.StartTime, ":", "")
		endTimeStr := strings.ReplaceAll(shift.EndTime, ":", "")

		dtStart := fmt.Sprintf("%sT%s00", dateStr, startTimeStr)
		dtEnd := fmt.Sprintf("%sT%s00", dateStr, endTimeStr)

		b.WriteString("BEGIN:VEVENT\r\n")
		b.WriteString(fmt.Sprintf("UID:%s@parashift\r\n", assignment.ID))
		b.WriteString(fmt.Sprintf("DTSTART:%s\r\n", dtStart))
		b.WriteString(fmt.Sprintf("DTEND:%s\r\n", dtEnd))
		b.WriteString("SUMMARY:Work Shift\r\n")
		b.WriteString("END:VEVENT\r\n")
	}

	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}
