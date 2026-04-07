package repo

import (
	"context"
	"errors"
	"time"

	"github.com/ovander/parashift/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type publicHolidayRepository struct {
	db *gorm.DB
}

// NewPublicHolidayRepository creates a new public holiday repository.
func NewPublicHolidayRepository(db *gorm.DB) PublicHolidayRepository {
	return &publicHolidayRepository{db: db}
}

func (r *publicHolidayRepository) ListByYear(ctx context.Context, year int, zone string) ([]*model.PublicHoliday, error) {
	var holidays []*model.PublicHoliday
	if err := r.db.WithContext(ctx).
		Where("EXTRACT(YEAR FROM date) = ? AND zone = ?", year, zone).
		Order("date ASC").
		Find(&holidays).Error; err != nil {
		return nil, err
	}
	return holidays, nil
}

func (r *publicHolidayRepository) GetByDate(ctx context.Context, date time.Time, zone string) (*model.PublicHoliday, error) {
	// Normalise to date-only for comparison.
	d := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	var h model.PublicHoliday
	if err := r.db.WithContext(ctx).
		Where("date = ? AND zone = ?", d, zone).
		First(&h).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &h, nil
}

func (r *publicHolidayRepository) UpsertBatch(ctx context.Context, holidays []*model.PublicHoliday) error {
	if len(holidays) == 0 {
		return nil
	}
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "date"}, {Name: "zone"}},
			DoUpdates: clause.AssignmentColumns([]string{"name"}),
		}).
		Create(&holidays).Error
}
