package repo

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// inviteTokenExpiry is the age after which an unclaimed claim token is "expired".
const inviteTokenExpiry = 7 * 24 * time.Hour

// employeeRepository implements EmployeeRepository.
type employeeRepository struct {
	db *gorm.DB
}

// NewEmployeeRepository creates a new employee repository.
func NewEmployeeRepository(db *gorm.DB) EmployeeRepository {
	return &employeeRepository{db: db}
}

// GetByID retrieves an employee by tenant and ID.
func (r *employeeRepository) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Employee, error) {
	var employee model.Employee
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		First(&employee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &employee, nil
}

// GetByAuthID retrieves an employee by their authentication ID (global lookup, no tenant scope).
func (r *employeeRepository) GetByAuthID(ctx context.Context, authID string) (*model.Employee, error) {
	var employee model.Employee
	if err := r.db.WithContext(ctx).
		Where("auth_id = ?", authID).
		First(&employee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &employee, nil
}

// GetByClaimToken retrieves an employee by their one-time invite token (global lookup).
func (r *employeeRepository) GetByClaimToken(ctx context.Context, token string) (*model.Employee, error) {
	var employee model.Employee
	if err := r.db.WithContext(ctx).
		Where("claim_token = ?", token).
		First(&employee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &employee, nil
}

// List retrieves all employees for a tenant with pagination.
func (r *employeeRepository) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error) {
	var employees []*model.Employee
	var total int64

	if err := r.db.WithContext(ctx).
		Model(&model.Employee{}).
		Where("tenant_id = ?", tenantID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ?", tenantID).
		Offset(offset).
		Limit(pageSize).
		Find(&employees).Error; err != nil {
		return nil, 0, err
	}

	return employees, total, nil
}

// ListAll retrieves employees cross-tenant with optional filtering and pagination.
// A LEFT JOIN on stores populates the virtual StoreName field.
func (r *employeeRepository) ListAll(ctx context.Context, filter EmployeeFilter, page, pageSize int) ([]*model.Employee, int64, error) {
	type row struct {
		model.Employee
		StoreName string `gorm:"column:store_name"`
	}

	q := r.db.WithContext(ctx).
		Table("employees e").
		Joins("LEFT JOIN stores s ON s.id = e.tenant_id AND s.deleted_at IS NULL").
		Where("e.deleted_at IS NULL")

	if filter.Position != "" {
		q = q.Where("e.position = ?", filter.Position)
	}
	if filter.JobRole != "" {
		q = q.Where("e.job_role = ?", filter.JobRole)
	}
	if filter.StoreID != nil {
		q = q.Where("e.tenant_id = ?", *filter.StoreID)
	}

	switch filter.Status {
	case "active":
		q = q.Where("e.auth_id <> ''")
	case "pending":
		q = q.Where("e.email <> '' AND e.auth_id = ''")
	case "unclaimed":
		q = q.Where("e.claim_token IS NOT NULL")
	}

	switch filter.Integrity {
	case "noStore":
		q = q.Where("s.id IS NULL OR s.deleted_at IS NOT NULL")
	case "noRole":
		q = q.Where("e.job_role = '' OR e.job_role IS NULL")
	case "expiredTokens":
		expiry := time.Now().Add(-inviteTokenExpiry)
		q = q.Where("e.claim_token IS NOT NULL AND e.created_at < ?", expiry)
	}

	var total int64
	if err := q.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	var rows []row
	if err := q.
		Select("e.*, s.name AS store_name").
		Offset(offset).
		Limit(pageSize).
		Order("e.created_at DESC").
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	employees := make([]*model.Employee, len(rows))
	for i, rr := range rows {
		emp := rr.Employee
		emp.StoreName = rr.StoreName
		employees[i] = &emp
	}
	return employees, total, nil
}

// GetByIDGlobal retrieves any employee by ID regardless of tenant.
func (r *employeeRepository) GetByIDGlobal(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	type row struct {
		model.Employee
		StoreName string `gorm:"column:store_name"`
	}
	var rr row
	if err := r.db.WithContext(ctx).
		Table("employees e").
		Joins("LEFT JOIN stores s ON s.id = e.tenant_id AND s.deleted_at IS NULL").
		Select("e.*, s.name AS store_name").
		Where("e.id = ? AND e.deleted_at IS NULL", id).
		Scan(&rr).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if rr.Employee.ID == uuid.Nil {
		return nil, nil
	}
	emp := rr.Employee
	emp.StoreName = rr.StoreName
	return &emp, nil
}

// GetByEmail retrieves an unlinked employee (auth_id='') matching the given email (global lookup).
// Returns nil, nil when not found. Used for auto-linking on first login.
func (r *employeeRepository) GetByEmail(ctx context.Context, email string) (*model.Employee, error) {
	var employee model.Employee
	if err := r.db.WithContext(ctx).
		Where("email = ? AND (auth_id = '' OR auth_id IS NULL) AND deleted_at IS NULL", email).
		First(&employee).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &employee, nil
}

// DeleteGlobal soft-deletes any employee by ID regardless of tenant.
func (r *employeeRepository) DeleteGlobal(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&model.Employee{}).Error
}

// Create creates a new employee.
func (r *employeeRepository) Create(ctx context.Context, e *model.Employee) error {
	return r.db.WithContext(ctx).Create(e).Error
}

// Update updates an existing employee.
func (r *employeeRepository) Update(ctx context.Context, e *model.Employee) error {
	return r.db.WithContext(ctx).Save(e).Error
}

// Delete deletes an employee by tenant and ID.
func (r *employeeRepository) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND id = ?", tenantID, id).
		Delete(&model.Employee{}).Error
}
