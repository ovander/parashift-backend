// Tests for AuditSubscriber.extractResource — the function that derives
// resource type and ID from each domain event payload.
//
// We use package "event" (white-box) so we can call the unexported extractResource
// directly, avoiding the need for a live *gorm.DB in unit tests.
package event

import (
	"testing"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/stretchr/testify/assert"
)

// ── Leave events ──────────────────────────────────────────────────────────────

func TestExtractResource_LeaveCreated(t *testing.T) {
	lr := &model.LeaveRequest{}
	lr.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeLeaveCreated, Payload: lr})
	assert.Equal(t, "leave_request", rType)
	assert.Equal(t, lr.ID.String(), rID)
}

func TestExtractResource_LeaveUpdated(t *testing.T) {
	lr := &model.LeaveRequest{}
	lr.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeLeaveUpdated, Payload: lr})
	assert.Equal(t, "leave_request", rType)
	assert.Equal(t, lr.ID.String(), rID)
}

// ── Assignment events ─────────────────────────────────────────────────────────

func TestExtractResource_AssignmentCreated(t *testing.T) {
	a := &model.ShiftAssignment{}
	a.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeAssignmentCreated, Payload: a})
	assert.Equal(t, "assignment", rType)
	assert.Equal(t, a.ID.String(), rID)
}

func TestExtractResource_AssignmentUpdated_Model(t *testing.T) {
	a := &model.ShiftAssignment{}
	a.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeAssignmentUpdated, Payload: a})
	assert.Equal(t, "assignment", rType)
	assert.Equal(t, a.ID.String(), rID)
}

func TestExtractResource_AssignmentUpdated_MapWithID(t *testing.T) {
	id := uuid.New()
	payload := map[string]interface{}{"id": id, "action": "deleted"}

	rType, rID := extractResource(Event{Type: TypeAssignmentUpdated, Payload: payload})
	assert.Equal(t, "assignment", rType)
	assert.Equal(t, id.String(), rID)
}

func TestExtractResource_AssignmentUpdated_BulkMap(t *testing.T) {
	// Bulk operations (e.g. reset_week) carry no single resource ID.
	payload := map[string]interface{}{"action": "reset_week", "week_start": "2026-04-07"}

	rType, rID := extractResource(Event{Type: TypeAssignmentUpdated, Payload: payload})
	assert.Equal(t, "assignment", rType)
	assert.Equal(t, "", rID, "bulk payloads without an id field should yield empty resource_id")
}

// ── Shift events ──────────────────────────────────────────────────────────────

func TestExtractResource_ShiftCreated(t *testing.T) {
	s := &model.ShiftInstance{}
	s.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeShiftCreated, Payload: s})
	assert.Equal(t, "shift", rType)
	assert.Equal(t, s.ID.String(), rID)
}

func TestExtractResource_ShiftUpdated(t *testing.T) {
	s := &model.ShiftInstance{}
	s.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeShiftUpdated, Payload: s})
	assert.Equal(t, "shift", rType)
	assert.Equal(t, s.ID.String(), rID)
}

func TestExtractResource_ShiftDeleted_MapWithID(t *testing.T) {
	id := uuid.New()
	payload := map[string]interface{}{"id": id}

	rType, rID := extractResource(Event{Type: TypeShiftDeleted, Payload: payload})
	assert.Equal(t, "shift", rType)
	assert.Equal(t, id.String(), rID)
}

func TestExtractResource_ShiftUpdated_BulkPublish(t *testing.T) {
	// "Published" events carry no single shift ID.
	payload := map[string]interface{}{"action": "published", "from": "2026-04-07"}

	rType, rID := extractResource(Event{Type: TypeShiftUpdated, Payload: payload})
	assert.Equal(t, "shift", rType)
	assert.Equal(t, "", rID)
}

// ── Swap events ───────────────────────────────────────────────────────────────

func TestExtractResource_SwapCreated(t *testing.T) {
	s := &model.SwapRequest{}
	s.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeSwapCreated, Payload: s})
	assert.Equal(t, "swap_request", rType)
	assert.Equal(t, s.ID.String(), rID)
}

func TestExtractResource_SwapUpdated(t *testing.T) {
	s := &model.SwapRequest{}
	s.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeSwapUpdated, Payload: s})
	assert.Equal(t, "swap_request", rType)
	assert.Equal(t, s.ID.String(), rID)
}

// ── Employee events ───────────────────────────────────────────────────────────

func TestExtractResource_EmployeeCreated(t *testing.T) {
	e := &model.Employee{}
	e.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeEmployeeCreated, Payload: e})
	assert.Equal(t, "employee", rType)
	assert.Equal(t, e.ID.String(), rID)
}

func TestExtractResource_EmployeeUpdated(t *testing.T) {
	e := &model.Employee{}
	e.ID = uuid.New()

	rType, rID := extractResource(Event{Type: TypeEmployeeUpdated, Payload: e})
	assert.Equal(t, "employee", rType)
	assert.Equal(t, e.ID.String(), rID)
}

// ── Unknown / nil payload ─────────────────────────────────────────────────────

func TestExtractResource_UnknownEventType(t *testing.T) {
	rType, rID := extractResource(Event{Type: "unknown.event", Payload: nil})
	assert.Equal(t, "", rType)
	assert.Equal(t, "", rID)
}

func TestExtractResource_WrongPayloadType(t *testing.T) {
	// Leave event with wrong payload type — must not panic.
	rType, rID := extractResource(Event{Type: TypeLeaveCreated, Payload: "not-a-leave-request"})
	assert.Equal(t, "", rType)
	assert.Equal(t, "", rID)
}
