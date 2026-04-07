package event

import (
	"github.com/google/uuid"
)

// EventType represents the type of event that occurred.
type EventType string

const (
	// TypeShiftCreated is fired when a new shift is created
	TypeShiftCreated EventType = "shift.created"
	// TypeShiftUpdated is fired when a shift is updated
	TypeShiftUpdated EventType = "shift.updated"
	// TypeShiftDeleted is fired when a shift is deleted
	TypeShiftDeleted EventType = "shift.deleted"
	// TypeAssignmentCreated is fired when a new assignment is created
	TypeAssignmentCreated EventType = "assignment.created"
	// TypeAssignmentUpdated is fired when an assignment is updated
	TypeAssignmentUpdated EventType = "assignment.updated"
	// TypeLeaveCreated is fired when a new leave request is created
	TypeLeaveCreated EventType = "leave.created"
	// TypeLeaveUpdated is fired when a leave request is updated
	TypeLeaveUpdated EventType = "leave.updated"
	// TypeSwapCreated is fired when a new swap request is created
	TypeSwapCreated EventType = "swap.created"
	// TypeSwapUpdated is fired when a swap request is updated
	TypeSwapUpdated EventType = "swap.updated"
	// TypeEmployeeCreated is fired when a new employee is created
	TypeEmployeeCreated EventType = "employee.created"
	// TypeEmployeeUpdated is fired when an employee is updated
	TypeEmployeeUpdated EventType = "employee.updated"
)

// Event represents a domain event in the system.
type Event struct {
	Type     EventType
	TenantID uuid.UUID
	UserID   uuid.UUID
	Payload  any
}
