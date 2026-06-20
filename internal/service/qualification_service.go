package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// QualificationService handles qualification management.
type QualificationService struct {
	qualRepo repo.QualificationRepository
	empQualRepo repo.EmployeeQualificationRepository
	logger   *logrus.Entry
}

// NewQualificationService creates a new qualification service.
func NewQualificationService(qualRepo repo.QualificationRepository, empQualRepo repo.EmployeeQualificationRepository, logger *logrus.Entry) *QualificationService {
	return &QualificationService{
		qualRepo:    qualRepo,
		empQualRepo: empQualRepo,
		logger:      logger,
	}
}

// ─── Qualification CRUD ──────────────────────────────────────────────────────

// ListQualifications returns all qualifications for a store.
func (s *QualificationService) ListQualifications(ctx context.Context, tenantID uuid.UUID) ([]dto.QualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"op":        "QualificationService.ListQualifications",
	}).Debug("listing qualifications")

	quals, err := s.qualRepo.List(ctx, tenantID)
	if err != nil {
		logger.WithError(err).Error("failed to list qualifications")
		return nil, apierror.Internal("failed to list qualifications").WithKey("errors.unknown")
	}

	logger.WithField("count", len(quals)).Debug("qualifications listed")

	responses := make([]dto.QualificationResponse, len(quals))
	for i, q := range quals {
		responses[i] = qualToResponse(q)
	}
	return responses, nil
}

// CreateQualification creates a new qualification.
func (s *QualificationService) CreateQualification(ctx context.Context, tenantID uuid.UUID, req dto.CreateQualificationRequest) (*dto.QualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"name":      req.Name,
		"op":        "QualificationService.CreateQualification",
	}).Info("creating qualification")

	if req.Name == "" {
		return nil, apierror.BadRequest("name is required").WithKey("errors.invalidInput")
	}

	qual := &model.Qualification{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:            req.Name,
		IssuingBody:     req.IssuingBody,
		RequiredForRole: req.RequiredForRole,
	}

	if err := s.qualRepo.Create(ctx, qual); err != nil {
		logger.WithError(err).Error("failed to create qualification")
		return nil, apierror.Internal("failed to create qualification").WithKey("errors.unknown")
	}

	logger.WithField("qual_id", qual.ID).Info("qualification created")

	resp := qualToResponse(qual)
	return &resp, nil
}

// UpdateQualification updates an existing qualification.
func (s *QualificationService) UpdateQualification(ctx context.Context, tenantID, id uuid.UUID, req dto.UpdateQualificationRequest) (*dto.QualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"qual_id":   id,
		"op":        "QualificationService.UpdateQualification",
	}).Info("updating qualification")

	qual, err := s.qualRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get qualification")
		return nil, apierror.Internal("failed to get qualification").WithKey("errors.unknown")
	}
	if qual == nil {
		logger.WithField("qual_id", id).Warn("qualification not found for update")
		return nil, apierror.NotFound("qualification", id.String()).WithKey("errors.unknown")
	}

	if req.Name != "" {
		qual.Name = req.Name
	}
	if req.IssuingBody != "" {
		qual.IssuingBody = req.IssuingBody
	}
	if req.RequiredForRole != "" {
		qual.RequiredForRole = req.RequiredForRole
	}

	qual.UpdatedAt = time.Now()

	if err := s.qualRepo.Update(ctx, qual); err != nil {
		logger.WithError(err).Error("failed to update qualification")
		return nil, apierror.Internal("failed to update qualification").WithKey("errors.unknown")
	}

	logger.WithField("qual_id", qual.ID).Info("qualification updated")

	resp := qualToResponse(qual)
	return &resp, nil
}

// DeleteQualification removes a qualification.
func (s *QualificationService) DeleteQualification(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"qual_id":   id,
		"op":        "QualificationService.DeleteQualification",
	}).Info("deleting qualification")

	qual, err := s.qualRepo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get qualification for delete")
		return apierror.Internal("failed to get qualification").WithKey("errors.unknown")
	}
	if qual == nil {
		logger.WithField("qual_id", id).Warn("qualification not found for delete")
		return apierror.NotFound("qualification", id.String()).WithKey("errors.unknown")
	}

	if err := s.qualRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete qualification")
		return apierror.Internal("failed to delete qualification").WithKey("errors.unknown")
	}

	logger.WithField("qual_id", id).Info("qualification deleted")
	return nil
}

// ─── Employee Qualification CRUD ─────────────────────────────────────────────

// ListEmployeeQualifications returns all qualifications held by an employee.
func (s *QualificationService) ListEmployeeQualifications(ctx context.Context, tenantID, employeeID uuid.UUID) ([]dto.EmployeeQualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":   tenantID,
		"employee_id": employeeID,
		"op":          "QualificationService.ListEmployeeQualifications",
	}).Debug("listing employee qualifications")

	empQuals, err := s.empQualRepo.ListByEmployee(ctx, tenantID, employeeID)
	if err != nil {
		logger.WithError(err).Error("failed to list employee qualifications")
		return nil, apierror.Internal("failed to list qualifications").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"employee_id": employeeID,
		"count":       len(empQuals),
	}).Debug("employee qualifications listed")

	responses := make([]dto.EmployeeQualificationResponse, len(empQuals))
	for i, eq := range empQuals {
		responses[i] = empQualToResponse(eq)
	}
	return responses, nil
}

// AddEmployeeQualification assigns a qualification to an employee.
func (s *QualificationService) AddEmployeeQualification(ctx context.Context, tenantID, employeeID uuid.UUID, req dto.AddEmployeeQualificationRequest) (*dto.EmployeeQualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id":        tenantID,
		"employee_id":      employeeID,
		"qualification_id": req.QualificationID,
		"op":               "QualificationService.AddEmployeeQualification",
	}).Info("adding qualification to employee")

	// Verify the qualification exists
	qual, err := s.qualRepo.GetByID(ctx, tenantID, req.QualificationID)
	if err != nil {
		logger.WithError(err).Error("failed to get qualification")
		return nil, apierror.Internal("failed to get qualification").WithKey("errors.unknown")
	}
	if qual == nil {
		logger.WithField("qualification_id", req.QualificationID).Warn("qualification not found")
		return nil, apierror.NotFound("qualification", req.QualificationID.String()).WithKey("errors.unknown")
	}

	var issueDate, expiryDate *time.Time

	if req.IssueDate != nil {
		t, err := time.Parse("2006-01-02", *req.IssueDate)
		if err != nil {
			return nil, apierror.BadRequest("invalid issue_date format (expected YYYY-MM-DD)").WithKey("errors.invalidInput")
		}
		issueDate = &t
	}

	if req.ExpiryDate != nil {
		t, err := time.Parse("2006-01-02", *req.ExpiryDate)
		if err != nil {
			return nil, apierror.BadRequest("invalid expiry_date format (expected YYYY-MM-DD)").WithKey("errors.invalidInput")
		}
		expiryDate = &t
	}

	empQual := &model.EmployeeQualification{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		EmployeeID:      employeeID,
		QualificationID: req.QualificationID,
		IssueDate:       issueDate,
		ExpiryDate:      expiryDate,
		DocumentURL:     req.DocumentURL,
	}

	if err := s.empQualRepo.Create(ctx, empQual); err != nil {
		logger.WithError(err).Error("failed to create employee qualification")
		return nil, apierror.Internal("failed to create qualification").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"eq_id":       empQual.ID,
		"employee_id": employeeID,
		"qual_id":     req.QualificationID,
	}).Info("employee qualification added")

	resp := empQualToResponseWithName(empQual, qual.Name)
	return &resp, nil
}

// UpdateEmployeeQualification updates an employee's qualification record.
func (s *QualificationService) UpdateEmployeeQualification(ctx context.Context, tenantID, id uuid.UUID, req dto.UpdateEmployeeQualificationRequest) (*dto.EmployeeQualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"eq_id":     id,
		"op":        "QualificationService.UpdateEmployeeQualification",
	}).Info("updating employee qualification")

	empQual, err := s.empQualRepo.ListByEmployee(ctx, tenantID, uuid.Nil)
	if err != nil {
		logger.WithError(err).Error("failed to list employee qualifications")
		return nil, apierror.Internal("failed to get qualification").WithKey("errors.unknown")
	}

	var found *model.EmployeeQualification
	for _, eq := range empQual {
		if eq.ID == id {
			found = eq
			break
		}
	}
	if found == nil {
		logger.WithField("eq_id", id).Warn("employee qualification not found")
		return nil, apierror.NotFound("qualification", id.String()).WithKey("errors.unknown")
	}

	if req.IssueDate != nil {
		t, err := time.Parse("2006-01-02", *req.IssueDate)
		if err != nil {
			return nil, apierror.BadRequest("invalid issue_date format (expected YYYY-MM-DD)").WithKey("errors.invalidInput")
		}
		found.IssueDate = &t
	}

	if req.ExpiryDate != nil {
		t, err := time.Parse("2006-01-02", *req.ExpiryDate)
		if err != nil {
			return nil, apierror.BadRequest("invalid expiry_date format (expected YYYY-MM-DD)").WithKey("errors.invalidInput")
		}
		found.ExpiryDate = &t
	}

	if req.Verified != nil {
		found.Verified = *req.Verified
	}

	if req.DocumentURL != "" {
		found.DocumentURL = req.DocumentURL
	}

	found.UpdatedAt = time.Now()

	if err := s.empQualRepo.Update(ctx, found); err != nil {
		logger.WithError(err).Error("failed to update employee qualification")
		return nil, apierror.Internal("failed to update qualification").WithKey("errors.unknown")
	}

	logger.WithField("eq_id", found.ID).Info("employee qualification updated")

	// Get the qualification name for the response
	qual, _ := s.qualRepo.GetByID(ctx, tenantID, found.QualificationID)
	qualName := ""
	if qual != nil {
		qualName = qual.Name
	}

	resp := empQualToResponseWithName(found, qualName)
	return &resp, nil
}

// RemoveEmployeeQualification removes a qualification from an employee.
func (s *QualificationService) RemoveEmployeeQualification(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"eq_id":     id,
		"op":        "QualificationService.RemoveEmployeeQualification",
	}).Info("removing employee qualification")

	if err := s.empQualRepo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete employee qualification")
		return apierror.Internal("failed to delete qualification").WithKey("errors.unknown")
	}

	logger.WithField("eq_id", id).Info("employee qualification removed")
	return nil
}

// ListExpiringQualifications returns qualifications expiring within N days.
func (s *QualificationService) ListExpiringQualifications(ctx context.Context, tenantID uuid.UUID, days int) ([]dto.EmployeeQualificationResponse, error) {
	logger := ctxutil.GetLogger(ctx)
	logger.WithFields(logrus.Fields{
		"tenant_id": tenantID,
		"days":      days,
		"op":        "QualificationService.ListExpiringQualifications",
	}).Debug("listing expiring qualifications")

	expiring, err := s.empQualRepo.ListExpiringWithinDays(ctx, tenantID, days)
	if err != nil {
		logger.WithError(err).Error("failed to list expiring qualifications")
		return nil, apierror.Internal("failed to list qualifications").WithKey("errors.unknown")
	}

	logger.WithFields(logrus.Fields{
		"days":  days,
		"count": len(expiring),
	}).Debug("expiring qualifications listed")

	responses := make([]dto.EmployeeQualificationResponse, len(expiring))
	for i, eq := range expiring {
		responses[i] = empQualToResponse(eq)
	}
	return responses, nil
}

// ValidateAssignmentQualification checks if an employee has a required qualification.
func (s *QualificationService) ValidateAssignmentQualification(ctx context.Context, tenantID, employeeID uuid.UUID, requiredQualID *uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)
	if requiredQualID == nil {
		return nil
	}

	logger.WithFields(logrus.Fields{
		"tenant_id":   tenantID,
		"employee_id": employeeID,
		"qual_id":     *requiredQualID,
		"op":          "QualificationService.ValidateAssignmentQualification",
	}).Debug("validating employee qualification for assignment")

	hasQual, err := s.empQualRepo.HasValidQualification(ctx, tenantID, employeeID, *requiredQualID)
	if err != nil {
		logger.WithError(err).Error("failed to check qualification")
		return apierror.Internal("failed to check qualification").WithKey("errors.unknown")
	}

	if !hasQual {
		logger.WithFields(logrus.Fields{
			"employee_id": employeeID,
			"qual_id":     *requiredQualID,
		}).Warn("assignment rejected: employee missing required qualification")
		return apierror.ValidationError("qualification_required", "employee does not have required qualification").WithKey("errors.conflict")
	}

	logger.WithFields(logrus.Fields{
		"employee_id": employeeID,
		"qual_id":     *requiredQualID,
	}).Debug("qualification validated: employee is qualified")
	return nil
}

// ─── Helper functions ────────────────────────────────────────────────────────

func qualToResponse(q *model.Qualification) dto.QualificationResponse {
	return dto.QualificationResponse{
		ID:              q.ID,
		Name:            q.Name,
		IssuingBody:     q.IssuingBody,
		RequiredForRole: q.RequiredForRole,
		CreatedAt:       q.CreatedAt.Format(time.RFC3339),
	}
}

func empQualToResponse(eq *model.EmployeeQualification) dto.EmployeeQualificationResponse {
	resp := dto.EmployeeQualificationResponse{
		ID:              eq.ID,
		EmployeeID:      eq.EmployeeID,
		QualificationID: eq.QualificationID,
		Verified:        eq.Verified,
		DocumentURL:     eq.DocumentURL,
		CreatedAt:       eq.CreatedAt.Format(time.RFC3339),
	}

	if eq.IssueDate != nil {
		issued := eq.IssueDate.Format("2006-01-02")
		resp.IssueDate = &issued
	}

	if eq.ExpiryDate != nil {
		expiry := eq.ExpiryDate.Format("2006-01-02")
		resp.ExpiryDate = &expiry

		daysUntilExpiry := int(time.Until(*eq.ExpiryDate).Hours() / 24)
		if daysUntilExpiry >= 0 && daysUntilExpiry <= 30 {
			resp.ExpiringInDays = &daysUntilExpiry
		}
	}

	return resp
}

func empQualToResponseWithName(eq *model.EmployeeQualification, qualName string) dto.EmployeeQualificationResponse {
	resp := empQualToResponse(eq)
	resp.QualificationName = qualName
	return resp
}
