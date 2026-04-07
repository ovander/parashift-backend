package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// contractRepository implements ContractRepository.
type contractRepository struct {
	db *gorm.DB
}

// NewContractRepository creates a new contract repository.
func NewContractRepository(db *gorm.DB) ContractRepository {
	return &contractRepository{db: db}
}

// GetByEmployeeID retrieves the contract for an employee.
func (r *contractRepository) GetByEmployeeID(ctx context.Context, tenantID, employeeID uuid.UUID) (*model.Contract, error) {
	var contract model.Contract
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		First(&contract).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &contract, nil
}

// Create creates a new contract.
func (r *contractRepository) Create(ctx context.Context, c *model.Contract) error {
	return r.db.WithContext(ctx).Create(c).Error
}

// Update updates an existing contract.
func (r *contractRepository) Update(ctx context.Context, c *model.Contract) error {
	return r.db.WithContext(ctx).Save(c).Error
}
