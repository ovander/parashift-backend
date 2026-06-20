package service

import (
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/stretchr/testify/assert"
)

// ARC-1: renderICS is pure and unit-testable without a DB. A well-formed calendar
// with one VEVENT per assignment that has a matching shift.
func TestRenderICS_WellFormedCalendar(t *testing.T) {
	shiftID := uuid.New()
	assignID := uuid.New()
	shiftByID := map[uuid.UUID]*model.ShiftInstance{
		shiftID: {
			TenantScoped: model.TenantScoped{ID: shiftID},
			Date:         time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC),
			StartTime:    "09:00",
			EndTime:      "17:30",
		},
	}
	assignments := []*model.ShiftAssignment{
		{TenantScoped: model.TenantScoped{ID: assignID}, ShiftInstanceID: shiftID},
	}

	ics := renderICS(assignments, shiftByID)

	assert.True(t, strings.HasPrefix(ics, "BEGIN:VCALENDAR\r\n"))
	assert.True(t, strings.HasSuffix(ics, "END:VCALENDAR\r\n"))
	assert.Contains(t, ics, "VERSION:2.0\r\n")
	assert.Contains(t, ics, "BEGIN:VEVENT\r\n")
	assert.Contains(t, ics, "UID:"+assignID.String()+"@parashift\r\n")
	assert.Contains(t, ics, "DTSTART:20260615T090000\r\n")
	assert.Contains(t, ics, "DTEND:20260615T173000\r\n")
	assert.Equal(t, 1, strings.Count(ics, "BEGIN:VEVENT"))
}

// ARC-1: assignments whose shift is missing are skipped (no event emitted).
func TestRenderICS_SkipsAssignmentsWithoutShift(t *testing.T) {
	assignments := []*model.ShiftAssignment{
		{TenantScoped: model.TenantScoped{ID: uuid.New()}, ShiftInstanceID: uuid.New()},
	}
	ics := renderICS(assignments, map[uuid.UUID]*model.ShiftInstance{})
	assert.NotContains(t, ics, "BEGIN:VEVENT")
}

// ARC-1: no assignments still yields a valid empty calendar.
func TestRenderICS_EmptyCalendar(t *testing.T) {
	ics := renderICS(nil, nil)
	assert.Equal(t, "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//ParaShift//WFM//EN\r\nCALSCALE:GREGORIAN\r\nMETHOD:PUBLISH\r\nEND:VCALENDAR\r\n", ics)
}
