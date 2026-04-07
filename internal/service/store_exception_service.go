package service

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
)

// StoreExceptionService manages one-off schedule exceptions for a store
// (e.g. exceptional Sunday opening, forced closure for inventory).
type StoreExceptionService struct {
	repo repo.StoreExceptionRepository
}

// NewStoreExceptionService creates a new StoreExceptionService.
func NewStoreExceptionService(r repo.StoreExceptionRepository) *StoreExceptionService {
	return &StoreExceptionService{repo: r}
}

// List returns all exceptions for a store within the given date range.
func (s *StoreExceptionService) List(ctx context.Context, tenantID uuid.UUID, from, to time.Time) ([]*model.StoreException, error) {
	return s.repo.ListByDateRange(ctx, tenantID, from, to)
}

// Create persists a new store exception.
func (s *StoreExceptionService) Create(ctx context.Context, exc *model.StoreException) error {
	return s.repo.Create(ctx, exc)
}

// Delete removes a store exception by ID.
func (s *StoreExceptionService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	return s.repo.Delete(ctx, tenantID, id)
}

// Repo exposes the underlying repository so ScheduleService can read exceptions
// during auto-projection without going through the service layer.
func (s *StoreExceptionService) Repo() repo.StoreExceptionRepository {
	return s.repo
}
