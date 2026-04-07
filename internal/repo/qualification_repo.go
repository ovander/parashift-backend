package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// qualificationRepository implements QualificationRepository.
type qualificationRepository struct {
	db *gorm.DB
}

// NewQualificationRepository creates a new qualification repository.
func NewQualificationRepository(db *gorm.DB) QualificationRepository {
	return &qualificationRepository{db: db}
}

// List retrieves all qualifications for a tenant.
func (r *qualificationRepository) List(ctx context.Context, tenantID uuid.UUID) ([]*model.Qualification, error) {
	var quals []*model.Qualification
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Find(&quals).Error; err != nil {
		return nil, err
	}
	return quals, nil
}

// GetByID retrieves a qualification by ID.
func (r *qualificationRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Qualification, error) {
	var qual model.Qualification
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&qual).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &qual, nil
}

// Create persists a new qualification.
func (r *qualificationRepository) Create(ctx context.Context, q *model.Qualification) error {
	return r.db.WithContext(ctx).Create(q).Error
}

// Update persists changes to a qualification.
func (r *qualificationRepository) Update(ctx context.Context, q *model.Qualification) error {
	return r.db.WithContext(ctx).Save(q).Error
}

// Delete removes a qualification.
func (r *qualificationRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.Qualification{}).Error
}

// ─── EmployeeQualificationRepository ──────────────────────────────────────

// employeeQualificationRepository implements EmployeeQualificationRepository.
type employeeQualificationRepository struct {
	db *gorm.DB
}

// NewEmployeeQualificationRepository creates a new employee qualification repository.
func NewEmployeeQualificationRepository(db *gorm.DB) EmployeeQualificationRepository {
	return &employeeQualificationRepository{db: db}
}

// ListByEmployee retrieves all qualifications held by an employee.
func (r *employeeQualificationRepository) ListByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.EmployeeQualification, error) {
	var eqs []*model.EmployeeQualification
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		Find(&eqs).Error; err != nil {
		return nil, err
	}
	return eqs, nil
}

// ListExpiringWithinDays retrieves qualifications expiring within N days.
func (r *employeeQualificationRepository) ListExpiringWithinDays(ctx context.Context, tenantID uuid.UUID, days int) ([]*model.EmployeeQualification, error) {
	var eqs []*model.EmployeeQualification
	cutoff := time.Now().AddDate(0, 0, days)
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND expiry_date IS NOT NULL AND expiry_date <= ? AND expiry_date > NOW()",
			tenantID, cutoff).
		Find(&eqs).Error; err != nil {
		return nil, err
	}
	return eqs, nil
}

// HasValidQualification checks if an employee holds a valid (non-expired) qualification.
func (r *employeeQualificationRepository) HasValidQualification(ctx context.Context, tenantID, employeeID, qualificationID uuid.UUID) (bool, error) {
	var count int64
	if err := r.db.WithContext(ctx).
		Model(&model.EmployeeQualification{}).
		Where("tenant_id = ? AND employee_id = ? AND qualification_id = ? AND (expiry_date IS NULL OR expiry_date > NOW())",
			tenantID, employeeID, qualificationID).
		Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}

// Create persists a new employee qualification.
func (r *employeeQualificationRepository) Create(ctx context.Context, eq *model.EmployeeQualification) error {
	return r.db.WithContext(ctx).Create(eq).Error
}

// Update persists changes to an employee qualification.
func (r *employeeQualificationRepository) Update(ctx context.Context, eq *model.EmployeeQualification) error {
	return r.db.WithContext(ctx).Save(eq).Error
}

// Delete removes an employee qualification.
func (r *employeeQualificationRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.EmployeeQualification{}).Error
}
