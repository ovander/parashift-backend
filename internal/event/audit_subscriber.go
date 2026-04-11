package event

import (
	"encoding/json"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// AuditSubscriber writes events to the audit log.
type AuditSubscriber struct {
	db *gorm.DB
}

// NewAuditSubscriber creates a new audit subscriber.
func NewAuditSubscriber(db *gorm.DB) *AuditSubscriber {
	return &AuditSubscriber{db: db}
}

// Handle processes an event and records it in the audit log.
func (as *AuditSubscriber) Handle(evt Event) {
	afterBytes, err := json.Marshal(evt.Payload)
	if err != nil {
		// Log error but don't block event processing
		return
	}

	resourceType, resourceID := extractResource(evt)

	auditLog := model.AuditLog{
		TenantID:     evt.TenantID,
		ActorID:      evt.UserID,
		Action:       string(evt.Type),
		ResourceType: resourceType,
		ResourceID:   resourceID,
		After:        afterBytes,
	}

	as.db.Create(&auditLog)
}

// extractResource derives the resource type and ID from the event payload using
// type assertions against the known model types. Falls back gracefully for
// bulk/map payloads where a single resource ID isn't applicable.
func extractResource(evt Event) (resourceType string, resourceID string) {
	switch evt.Type {

	case TypeLeaveCreated, TypeLeaveUpdated:
		if l, ok := evt.Payload.(*model.LeaveRequest); ok {
			return "leave_request", l.ID.String()
		}

	case TypeLeaveDeleted:
		if m, ok := evt.Payload.(map[string]interface{}); ok {
			if id, ok := m["id"].(uuid.UUID); ok {
				return "leave_request", id.String()
			}
		}
		return "leave_request", ""

	case TypeAssignmentCreated, TypeAssignmentUpdated:
		if a, ok := evt.Payload.(*model.ShiftAssignment); ok {
			return "assignment", a.ID.String()
		}
		// Bulk / map payloads (reset_week, deleted by ID)
		if m, ok := evt.Payload.(map[string]interface{}); ok {
			if id, ok := m["id"].(uuid.UUID); ok {
				return "assignment", id.String()
			}
		}
		return "assignment", ""

	case TypeShiftCreated, TypeShiftUpdated, TypeShiftDeleted:
		if s, ok := evt.Payload.(*model.ShiftInstance); ok {
			return "shift", s.ID.String()
		}
		if m, ok := evt.Payload.(map[string]interface{}); ok {
			if id, ok := m["id"].(uuid.UUID); ok {
				return "shift", id.String()
			}
		}
		return "shift", ""

	case TypeScheduleGenerated:
		return "schedule", ""

	case TypeSwapCreated, TypeSwapUpdated:
		if s, ok := evt.Payload.(*model.SwapRequest); ok {
			return "swap_request", s.ID.String()
		}

	case TypeEmployeeCreated, TypeEmployeeUpdated:
		if e, ok := evt.Payload.(*model.Employee); ok {
			return "employee", e.ID.String()
		}

	case TypeEmployeeDeleted:
		if m, ok := evt.Payload.(map[string]interface{}); ok {
			if id, ok := m["id"].(uuid.UUID); ok {
				return "employee", id.String()
			}
		}
		return "employee", ""

	case TypeStoreCreated, TypeStoreUpdated:
		if s, ok := evt.Payload.(*model.Store); ok {
			return "store", s.ID.String()
		}
	}

	return "", ""
}
