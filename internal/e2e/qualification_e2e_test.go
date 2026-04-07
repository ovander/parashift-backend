package e2e_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/testutil"
)

// TestE2E_Qualification_List tests listing qualifications.
func TestE2E_Qualification_List(t *testing.T) {
	mocks := emptyMocks()

	mocks.qualification.ListFn = func(ctx context.Context, tenantID uuid.UUID) ([]*model.Qualification, error) {
		return []*model.Qualification{}, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "GET", "/stores/"+storeID.String()+"/qualifications", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var listResp dto.QualificationListResponse
	decode(t, resp, &listResp)
	if len(listResp.Qualifications) != 0 {
		t.Fatalf("expected 0 qualifications, got %d", len(listResp.Qualifications))
	}
}

// TestE2E_Qualification_Create tests creating a qualification.
func TestE2E_Qualification_Create(t *testing.T) {
	mocks := emptyMocks()

	qualID := uuid.New()
	var created *model.Qualification

	mocks.qualification.CreateFn = func(ctx context.Context, q *model.Qualification) error {
		created = q
		return nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	req := dto.CreateQualificationRequest{
		Name:        "Pharmacist License",
		IssuingBody: "State Board",
	}

	resp := client.do(t, "POST", "/stores/"+storeID.String()+"/qualifications", req)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var qResp dto.QualificationResponse
	decode(t, resp, &qResp)

	if qResp.Name != "Pharmacist License" {
		t.Fatalf("expected name Pharmacist License, got %s", qResp.Name)
	}

	if created == nil {
		t.Fatalf("qualification not created in mock")
	}

	_ = qualID
}

// TestE2E_Qualification_Delete tests deleting a qualification.
func TestE2E_Qualification_Delete(t *testing.T) {
	mocks := emptyMocks()

	qualID := uuid.New()
	qual := &model.Qualification{
		TenantScoped: model.TenantScoped{
			ID:        qualID,
			TenantID:  uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:        "Pharmacist License",
		IssuingBody: "State Board",
	}

	mocks.qualification.GetByIDFn = func(ctx context.Context, tenantID, id uuid.UUID) (*model.Qualification, error) {
		if id == qualID {
			return qual, nil
		}
		return nil, nil
	}

	mocks.qualification.DeleteFn = func(ctx context.Context, tenantID, id uuid.UUID) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := qual.TenantID
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "DELETE", "/stores/"+storeID.String()+"/qualifications/"+qualID.String(), nil)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

// TestE2E_EmployeeQualification_Add tests adding a qualification to an employee.
func TestE2E_EmployeeQualification_Add(t *testing.T) {
	mocks := emptyMocks()

	qualID := uuid.New()
	qual := &model.Qualification{
		TenantScoped: model.TenantScoped{
			ID:        qualID,
			TenantID:  uuid.New(),
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name: "Pharmacist License",
	}

	mocks.qualification.GetByIDFn = func(ctx context.Context, tenantID, id uuid.UUID) (*model.Qualification, error) {
		if id == qualID {
			return qual, nil
		}
		return nil, nil
	}

	mocks.employeeQualification.CreateFn = func(ctx context.Context, eq *model.EmployeeQualification) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := qual.TenantID
	userID := uuid.New()
	employeeID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	req := dto.AddEmployeeQualificationRequest{
		QualificationID: qualID,
	}

	resp := client.do(t, "POST",
		"/stores/"+storeID.String()+"/employees/"+employeeID.String()+"/qualifications", req)

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var eqResp dto.EmployeeQualificationResponse
	decode(t, resp, &eqResp)

	if eqResp.QualificationID != qualID {
		t.Fatalf("expected qualification ID %s, got %s", qualID, eqResp.QualificationID)
	}
}

// TestE2E_EmployeeQualification_List tests listing an employee's qualifications.
func TestE2E_EmployeeQualification_List(t *testing.T) {
	mocks := emptyMocks()

	employeeID := uuid.New()

	mocks.employeeQualification.ListByEmployeeFn = func(ctx context.Context, tenantID, empID uuid.UUID) ([]*model.EmployeeQualification, error) {
		if empID == employeeID {
			return []*model.EmployeeQualification{}, nil
		}
		return nil, nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "GET",
		"/stores/"+storeID.String()+"/employees/"+employeeID.String()+"/qualifications", nil)

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var listResp dto.EmployeeQualificationListResponse
	decode(t, resp, &listResp)
	if len(listResp.Qualifications) != 0 {
		t.Fatalf("expected 0 qualifications, got %d", len(listResp.Qualifications))
	}
}

// TestE2E_EmployeeQualification_Remove tests removing a qualification from an employee.
func TestE2E_EmployeeQualification_Remove(t *testing.T) {
	mocks := emptyMocks()

	eqID := uuid.New()

	mocks.employeeQualification.DeleteFn = func(ctx context.Context, tenantID, id uuid.UUID) error {
		return nil
	}

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	employeeID := uuid.New()
	storeID := tenantID

	withManagerEmp(mocks, tenantID, "user123")
	client := newClient(ts, tenantID, userID, "user123", "manager")

	resp := client.do(t, "DELETE",
		"/stores/"+storeID.String()+"/employees/"+employeeID.String()+"/qualifications/"+eqID.String(), nil)

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}
}

// TestE2E_Qualification_Employee_Forbidden tests that employees cannot create qualifications.
func TestE2E_Qualification_Employee_Forbidden(t *testing.T) {
	mocks := emptyMocks()

	ts := newTestServer(t, mocks)
	defer ts.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	storeID := tenantID

	// Client with employee role
	empUser := testutil.NewEmployee(tenantID)
	empUser.AuthID = "user123"
	empUser.Position = "employee"
	mocks.emp.GetByAuthIDFn = func(_ context.Context, authID string) (*model.Employee, error) {
		if authID == "user123" {
			return empUser, nil
		}
		return nil, nil
	}
	client := newClient(ts, tenantID, userID, "user123", "employee")

	req := dto.CreateQualificationRequest{
		Name: "Test Qual",
	}

	resp := client.do(t, "POST", "/stores/"+storeID.String()+"/qualifications", req)

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}
