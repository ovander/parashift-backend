package repo

import (
	"context"
	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
)

// weekTemplateRepository implements WeekTemplateRepository.
type weekTemplateRepository struct {
	db *gorm.DB
}

// NewWeekTemplateRepository creates a new week template repository.
func NewWeekTemplateRepository(db *gorm.DB) WeekTemplateRepository {
	return &weekTemplateRepository{db: db}
}

// GetByEmployee retrieves all templates for an employee.
func (r *weekTemplateRepository) GetByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) ([]*model.WeekTemplate, error) {
	var templates []*model.WeekTemplate
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		Find(&templates).Error; err != nil {
		return nil, err
	}
	return templates, nil
}

// UpsertForEmployee replaces all templates for an employee using a transaction.
func (r *weekTemplateRepository) UpsertForEmployee(ctx context.Context, tenantID, employeeID uuid.UUID, templates []*model.WeekTemplate) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Delete existing templates for this employee
		if err := tx.Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
			Delete(&model.WeekTemplate{}).Error; err != nil {
			return err
		}

		// Set tenant_id and employee_id for all templates
		for _, template := range templates {
			template.TenantID = tenantID
			template.EmployeeID = employeeID
		}

		// Create new templates
		if len(templates) > 0 {
			if err := tx.CreateInBatches(templates, 100).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

// DeleteByEmployee deletes all templates for an employee.
func (r *weekTemplateRepository) DeleteByEmployee(ctx context.Context, tenantID, employeeID uuid.UUID) error {
	return r.db.WithContext(ctx).
		Where("tenant_id = ? AND employee_id = ?", tenantID, employeeID).
		Delete(&model.WeekTemplate{}).Error
}
