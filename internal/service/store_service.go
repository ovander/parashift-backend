package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
	"gorm.io/datatypes"
)

// StoreService manages stores (tenants).
// Admins can manage all stores; managers see their own store via JWT store_id.
type StoreService struct {
	repo    repo.StoreRepository
	emitter *event.Emitter
	logger  *logrus.Entry
}

// NewStoreService creates a new StoreService.
func NewStoreService(repo repo.StoreRepository, emitter *event.Emitter, logger *logrus.Entry) *StoreService {
	return &StoreService{
		repo:    repo,
		emitter: emitter,
		logger:  logger,
	}
}

// GetByID retrieves a store by ID.
func (s *StoreService) GetByID(ctx context.Context, id uuid.UUID) (*model.Store, error) {
	logger := ctxutil.GetLogger(ctx)
	store, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("failed to get store")
		return nil, apierror.Internal("failed to get store")
	}
	if store == nil {
		return nil, apierror.NotFound("store", id.String())
	}
	return store, nil
}

// List retrieves all stores with pagination (admin only, no tenant scoping).
func (s *StoreService) List(ctx context.Context, page, pageSize int) ([]*model.Store, int64, error) {
	logger := ctxutil.GetLogger(ctx)
	stores, total, err := s.repo.List(ctx, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list stores")
		return nil, 0, apierror.Internal("failed to list stores")
	}
	return stores, total, nil
}

// Create creates a new store.
func (s *StoreService) Create(ctx context.Context, req dto.CreateStoreRequest) (*model.Store, error) {
	logger := ctxutil.GetLogger(ctx)

	if req.Name == "" {
		return nil, apierror.BadRequest("store name is required")
	}

	store := &model.Store{
		ID:        uuid.New(),
		Name:      req.Name,
		Timezone:  req.Timezone,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if len(req.OpeningHours) > 0 {
		data, err := serializeOpeningHours(req.OpeningHours)
		if err != nil {
			logger.WithError(err).Error("failed to serialize opening hours")
			return nil, apierror.BadRequest("invalid opening hours format")
		}
		store.OpeningHours = data
	}

	if err := s.repo.Create(ctx, store); err != nil {
		logger.WithError(err).Error("failed to create store")
		return nil, apierror.Internal("failed to create store")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeEmployeeCreated,
		TenantID: store.ID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  store,
	})

	return store, nil
}

// Update updates an existing store.
func (s *StoreService) Update(ctx context.Context, id uuid.UUID, req dto.UpdateStoreRequest) (*model.Store, error) {
	logger := ctxutil.GetLogger(ctx)

	store, err := s.repo.GetByID(ctx, id)
	if err != nil {
		logger.WithError(err).Error("failed to get store for update")
		return nil, apierror.Internal("failed to get store")
	}
	if store == nil {
		return nil, apierror.NotFound("store", id.String())
	}

	if req.Name != nil {
		store.Name = *req.Name
	}
	if req.Timezone != nil {
		store.Timezone = *req.Timezone
	}
	if len(req.OpeningHours) > 0 {
		data, err := serializeOpeningHours(req.OpeningHours)
		if err != nil {
			logger.WithError(err).Error("failed to serialize opening hours")
			return nil, apierror.BadRequest("invalid opening hours format")
		}
		store.OpeningHours = data
	}

	store.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, store); err != nil {
		logger.WithError(err).Error("failed to update store")
		return nil, apierror.Internal("failed to update store")
	}

	return store, nil
}

// Delete deletes a store by ID.
func (s *StoreService) Delete(ctx context.Context, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	if err := s.repo.Delete(ctx, id); err != nil {
		logger.WithError(err).Error("failed to delete store")
		return apierror.Internal("failed to delete store")
	}
	return nil
}

// serializeOpeningHours marshals opening hour slots to datatypes.JSON.
func serializeOpeningHours(hours []dto.OpeningHourSlot) (datatypes.JSON, error) {
	data, err := json.Marshal(hours)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(data), nil
}
