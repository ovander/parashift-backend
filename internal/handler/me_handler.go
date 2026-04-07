package handler

import (
	"net/http"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/pagination"
	"github.com/ovander/parashift/internal/pkg"
	"github.com/ovander/parashift/internal/service"
)

// MeHandler handles user profile and personal schedule endpoints.
type MeHandler struct {
	empSvc *service.EmployeeService
	schSvc *service.ScheduleService
}

// NewMeHandler creates a new MeHandler.
func NewMeHandler(empSvc *service.EmployeeService, schSvc *service.ScheduleService) *MeHandler {
	return &MeHandler{
		empSvc: empSvc,
		schSvc: schSvc,
	}
}

// MeResponse is the shape returned by GET /me.
// For platform admins the store_id is omitted; for managers and employees it is always set.
type MeResponse struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Email    string  `json:"email,omitempty"`
	Position string  `json:"position"` // "admin" | "manager" | "employee" — RBAC access level
	JobRole  string  `json:"job_role"`  // pharmacist|animator|... — shift eligibility
	StoreID  *string `json:"store_id,omitempty"`
}

// GetProfile returns the authenticated user's profile.
//
// Socrate only knows two roles: "admin" and "user".
// ParaShift enriches "user" tokens with the employee's ParaShift role and store.
// "admin" tokens are served directly from JWT context — no DB lookup needed.
func (h *MeHandler) GetProfile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	socrateRole := ctxutil.GetUserRole(ctx)
	userSub  := ctxutil.GetUserSub(ctx)
	userName := ctxutil.GetUserName(ctx)
	userEmail := ctxutil.GetUserEmail(ctx)

	// Platform admin: identity comes entirely from the Socrate JWT.
	if socrateRole == "admin" {
		name := userName
		if name == "" {
			name = "Admin"
		}
		pkg.WriteJSON(w, http.StatusOK, MeResponse{
			ID:       userSub,
			Name:     name,
			Email:    userEmail,
			Position: "admin",
		})
		return
	}

	// Regular Socrate user → look up employee in DB to get ParaShift role + store.
	// The service returns apierror.NotFound when the sub has no employee record, and
	// apierror.Internal on DB failure — pkg.WriteError preserves the right status for both.
	employee, err := h.empSvc.GetByAuthID(ctx, userSub)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	storeID := employee.TenantID.String()
	pkg.WriteJSON(w, http.StatusOK, MeResponse{
		ID:       employee.ID.String(),
		Name:     employee.Name,
		Email:    userEmail,
		Position: employee.Position,
		JobRole:  employee.JobRole,
		StoreID:  &storeID,
	})
}

// GetMySchedule returns the authenticated user's assigned shifts for a date range.
func (h *MeHandler) GetMySchedule(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userSub := ctxutil.GetUserSub(ctx)

	employee, err := h.empSvc.GetByAuthID(ctx, userSub)
	if err != nil {
		pkg.WriteError(w, apierror.NotFound("employee", userSub))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		pkg.WriteError(w, apierror.BadRequest("from and to query parameters are required"))
		return
	}

	from, err := parseDate(fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD"))
		return
	}

	to, err := parseDate(toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD"))
		return
	}

	// ListMyShifts returns enriched shift+assignment data (service.ShiftWithAssignment)
	shifts, err := h.schSvc.ListMyShifts(ctx, employee.TenantID, employee.ID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	params := pagination.Parse(r)
	pkg.WriteJSON(w, http.StatusOK, pagination.NewPagedResponse(shifts, params, int64(len(shifts))))
}

// ExportICS exports the authenticated user's schedule as an ICS calendar file.
func (h *MeHandler) ExportICS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userSub := ctxutil.GetUserSub(ctx)

	employee, err := h.empSvc.GetByAuthID(ctx, userSub)
	if err != nil {
		pkg.WriteError(w, apierror.NotFound("employee", userSub))
		return
	}

	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")

	if fromStr == "" || toStr == "" {
		pkg.WriteError(w, apierror.BadRequest("from and to query parameters are required"))
		return
	}

	from, err := parseDate(fromStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid from date format, use YYYY-MM-DD"))
		return
	}

	to, err := parseDate(toStr)
	if err != nil {
		pkg.WriteError(w, apierror.BadRequest("invalid to date format, use YYYY-MM-DD"))
		return
	}

	// Service: GenerateICS(ctx, tenantID, employeeID, from, to)
	icsData, err := h.schSvc.GenerateICS(ctx, employee.TenantID, employee.ID, from, to)
	if err != nil {
		pkg.WriteError(w, err)
		return
	}

	w.Header().Set("Content-Type", "text/calendar")
	w.Header().Set("Content-Disposition", "attachment; filename=\"schedule.ics\"")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(icsData))
}
