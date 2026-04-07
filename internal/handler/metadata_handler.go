package handler

import (
	"net/http"

	"github.com/ovander/parashift/internal/pkg"
)

// MetadataHandler serves static schema information so the frontend
// never needs to hardcode enum values or business rules.
type MetadataHandler struct{}

// NewMetadataHandler creates a new MetadataHandler.
func NewMetadataHandler() *MetadataHandler {
	return &MetadataHandler{}
}

// employeeRoleMeta describes a single employee role.
type employeeRoleMeta struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// employeeStatusMeta describes an onboarding status filter value.
type employeeStatusMeta struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// integrityFilterMeta describes a data-integrity filter value.
type integrityFilterMeta struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
}

// metadataResponse is the full metadata payload.
type metadataResponse struct {
	EmployeeRoles    []employeeRoleMeta    `json:"employeeRoles"`
	EmployeeStatuses []employeeStatusMeta  `json:"employeeStatuses"`
	IntegrityFilters []integrityFilterMeta `json:"integrityFilters"`
}

var staticMetadata = metadataResponse{
	EmployeeRoles: []employeeRoleMeta{
		{Value: "manager", Label: "Manager", Description: "Can manage schedules, leaves, and store employees"},
		{Value: "pharmacist", Label: "Pharmacist", Description: "Licensed pharmacist — eligible for pharmacist-only shifts"},
		{Value: "employee", Label: "Employee", Description: "Standard employee with no management permissions"},
	},
	EmployeeStatuses: []employeeStatusMeta{
		{Value: "active", Label: "Active", Description: "Socrate account bound (auth_id present)"},
		{Value: "pending", Label: "Pending invite", Description: "Email invite sent, waiting for first login"},
		{Value: "unclaimed", Label: "Unclaimed link", Description: "Manual invite link generated but not yet used"},
	},
	IntegrityFilters: []integrityFilterMeta{
		{Value: "noStore", Label: "No store", Description: "Employee's store has been soft-deleted"},
		{Value: "noRole", Label: "No role", Description: "Employee has an empty role field"},
		{Value: "expiredTokens", Label: "Expired invites", Description: "Unclaimed invite links older than 7 days"},
	},
}

// GetMetadata returns all static metadata for the admin UI.
//
//	GET /api/v1/admin/metadata
func (h *MetadataHandler) GetMetadata(w http.ResponseWriter, r *http.Request) {
	pkg.WriteJSON(w, http.StatusOK, staticMetadata)
}
