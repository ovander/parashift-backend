package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/model"
)

// AuthCtx returns a context pre-loaded with the standard auth values used by handlers.
func AuthCtx(tenantID, userID uuid.UUID, sub, role string) context.Context {
	ctx := context.Background()
	ctx = ctxutil.WithTenantID(ctx, tenantID)
	ctx = ctxutil.WithUserID(ctx, userID)
	ctx = ctxutil.WithUserSub(ctx, sub)
	ctx = ctxutil.WithUserRole(ctx, role)
	return ctx
}

// NewRequest builds an *http.Request with a JSON body and auth context.
// If body is nil the request has no body.
func NewRequest(method, path string, body any, ctx context.Context) *http.Request {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if ctx != nil {
		req = req.WithContext(ctx)
	}
	return req
}

// MustJSON decodes the response body into v, panicking on error (test helper).
func MustJSON(rr *httptest.ResponseRecorder, v any) {
	if err := json.NewDecoder(rr.Body).Decode(v); err != nil {
		panic("testutil.MustJSON: " + err.Error())
	}
}

// NewEmployee returns a minimal *model.Employee for use in tests.
func NewEmployee(tenantID uuid.UUID) *model.Employee {
	return &model.Employee{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:     "Alice Dumont",
		Position: "employee",
		JobRole:  "pharmacist",
		AuthID:   "sub-alice-123",
		StartDate: time.Now().AddDate(-1, 0, 0),
	}
}

// NewStore returns a minimal *model.Store for use in tests.
func NewStore() *model.Store {
	return &model.Store{
		ID:        uuid.New(),
		Name:      "Pharmacie du Centre",
		Timezone:  "Europe/Brussels",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// NewShiftInstance returns a minimal *model.ShiftInstance.
func NewShiftInstance(tenantID uuid.UUID) *model.ShiftInstance {
	return &model.ShiftInstance{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Date:      time.Now(),
		StartTime: "09:00",
		EndTime:   "17:00",
		Role:      "pharmacist",
		Source:    model.SourceManual,
	}
}

// NewShiftAssignment returns a minimal *model.ShiftAssignment.
func NewShiftAssignment(tenantID, shiftID, employeeID uuid.UUID) *model.ShiftAssignment {
	return &model.ShiftAssignment{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		ShiftInstanceID: shiftID,
		EmployeeID:      employeeID,
		Status:          model.AssignmentStatusConfirmed,
		AssignedBy:      uuid.New(),
		AssignedAt:      time.Now(),
		// Denormalized shift time fields — use sensible defaults for tests.
		ShiftDate:      time.Now().Truncate(24 * time.Hour),
		ShiftStartTime: "09:00",
		ShiftEndTime:   "17:00",
	}
}

// NewLeaveRequest returns a minimal *model.LeaveRequest.
func NewLeaveRequest(tenantID, employeeID uuid.UUID) *model.LeaveRequest {
	return &model.LeaveRequest{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		EmployeeID: employeeID,
		StartDate:  time.Now(),
		EndDate:    time.Now().AddDate(0, 0, 5),
		Type:       model.LeaveTypeVacation,
		Status:     model.LeaveStatusPending,
		Reason:     "Annual leave",
	}
}

// NewAvailability returns a minimal *model.Availability.
func NewAvailability(tenantID, employeeID uuid.UUID) *model.Availability {
	return &model.Availability{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		EmployeeID: employeeID,
		Date:       time.Now(),
		TimeRanges: []byte(`[{"start":"09:00","end":"17:00"}]`),
		Note:       "Available morning",
	}
}

// NewCoverageRequirement returns a minimal *model.CoverageRequirement.
func NewCoverageRequirement(tenantID uuid.UUID) *model.CoverageRequirement {
	return &model.CoverageRequirement{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		DayOfWeek:    1,
		StartTime:    "09:00",
		EndTime:      "17:00",
		MinStaff:     2,
		RequiredRole: "pharmacist",
	}
}

// NewShiftSlot returns a minimal *model.ShiftSlot for use in tests.
func NewShiftSlot(tenantID uuid.UUID, scheme string, dayOfWeek int) *model.ShiftSlot {
	return &model.ShiftSlot{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Scheme:       scheme,
		DayOfWeek:    dayOfWeek,
		StartTime:    "08:00",
		EndTime:      "16:00",
		RequiredRole: "pharmacist",
	}
}

// NewStoreException returns a minimal *model.StoreException of the given type.
func NewStoreException(tenantID uuid.UUID, date time.Time, exType string) *model.StoreException {
	return &model.StoreException{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Date: date,
		Type: exType,
		Note: "test exception",
	}
}

// NewSwapRequest returns a minimal *model.SwapRequest.
func NewSwapRequest(tenantID, requesterID, shiftID uuid.UUID) *model.SwapRequest {
	return &model.SwapRequest{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		RequesterID:     requesterID,
		ShiftInstanceID: shiftID,
		Status:          model.SwapStatusPending,
		Note:            "Please swap with me",
	}
}
