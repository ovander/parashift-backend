package service

import (
	"context"
	"time"

	"github.com/ovander/backendkit/apierror"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

// AdminDashboardService computes the admin dashboard payload in a single pass.
// All queries are cross-tenant; no per-store filtering is applied.
type AdminDashboardService struct {
	db     *gorm.DB
	logger *logrus.Entry
}

// NewAdminDashboardService creates a new AdminDashboardService.
func NewAdminDashboardService(db *gorm.DB, logger *logrus.Entry) *AdminDashboardService {
	return &AdminDashboardService{db: db, logger: logger}
}

// ── Response types ────────────────────────────────────────────────────────────

// DashboardOrganization holds store-level aggregates.
type DashboardOrganization struct {
	TotalStores    int64 `json:"totalStores"`
	ActiveStores   int64 `json:"activeStores"`   // stores with ≥1 active (bound) employee
	InactiveStores int64 `json:"inactiveStores"` // totalStores - activeStores
}

// DashboardUsers holds employee onboarding aggregates.
type DashboardUsers struct {
	TotalEmployees  int64 `json:"totalEmployees"`
	TotalManagers   int64 `json:"totalManagers"`   // employees with role=manager
	ActiveEmployees int64 `json:"activeEmployees"` // auth_id bound (can log in)
	PendingInvites  int64 `json:"pendingInvites"`  // email set, auth_id still empty
	Unclaimed       int64 `json:"unclaimed"`       // claim_token present (manual invite not yet used)
}

// DashboardIntegrity holds data-quality issue counts.
type DashboardIntegrity struct {
	NoStore        int64 `json:"noStore"`        // employees whose store was soft-deleted
	NoRole         int64 `json:"noRole"`         // employees with empty role
	ExpiredTokens  int64 `json:"expiredTokens"`  // claim tokens older than 7 days
}

// RecentStore is a lightweight store record for the activity feed.
type RecentStore struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	CreatedAt string `json:"createdAt"`
}

// RecentInvite is a lightweight invite record for the activity feed.
type RecentInvite struct {
	EmployeeID string `json:"employeeId"`
	Email      string `json:"email"`
	CreatedAt  string `json:"createdAt"`
}

// DashboardActivity holds the most recent stores and invites.
type DashboardActivity struct {
	RecentStores  []RecentStore  `json:"recentStores"`
	RecentInvites []RecentInvite `json:"recentInvites"`
}

// AdminDashboard is the full dashboard payload returned to the client.
type AdminDashboard struct {
	Organization DashboardOrganization `json:"organization"`
	Users        DashboardUsers        `json:"users"`
	Integrity    DashboardIntegrity    `json:"integrity"`
	Activity     DashboardActivity     `json:"activity"`
}

// ── Query helpers ─────────────────────────────────────────────────────────────

// inviteExpiryThreshold is the age after which an unclaimed token is considered expired.
const inviteExpiryThreshold = 7 * 24 * time.Hour

// ── GetDashboard ──────────────────────────────────────────────────────────────

// GetDashboard aggregates all dashboard metrics. Each group of related counts is
// batched into as few queries as possible to keep latency well below 300 ms.
func (s *AdminDashboardService) GetDashboard(ctx context.Context) (*AdminDashboard, error) {

	// ── 1. Organization ───────────────────────────────────────────────────────

	var totalStores int64
	if err := s.db.WithContext(ctx).
		Table("stores").
		Where("deleted_at IS NULL").
		Count(&totalStores).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: count stores")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// An "active" store has at least one employee with a non-empty auth_id.
	// We use a subquery to avoid scanning the full employees table repeatedly.
	var activeStores int64
	if err := s.db.WithContext(ctx).
		Table("stores").
		Where("deleted_at IS NULL").
		Where(`id IN (
			SELECT DISTINCT tenant_id FROM employees
			WHERE deleted_at IS NULL AND auth_id <> ''
		)`).
		Count(&activeStores).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: count active stores")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// ── 2. Users / onboarding ─────────────────────────────────────────────────

	// Single scan over employees for all four employee counts.
	var ec struct {
		Total    int64 `gorm:"column:total"`
		Managers int64 `gorm:"column:managers"`
		Active   int64 `gorm:"column:active"`
		Pending  int64 `gorm:"column:pending"`
		Unclaim  int64 `gorm:"column:unclaim"`
	}
	if err := s.db.WithContext(ctx).
		Table("employees").
		Where("deleted_at IS NULL").
		Select(`
			COUNT(*)                                                      AS total,
			COUNT(*) FILTER (WHERE position = 'manager')                 AS managers,
			COUNT(*) FILTER (WHERE auth_id <> '')                        AS active,
			COUNT(*) FILTER (WHERE email <> '' AND auth_id = '')         AS pending,
			COUNT(*) FILTER (WHERE claim_token IS NOT NULL)              AS unclaim
		`).
		Scan(&ec).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: employee counts")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// ── 3. Integrity ──────────────────────────────────────────────────────────

	expiryDate := time.Now().Add(-inviteExpiryThreshold)

	// noStore: employees whose store (tenant) has been soft-deleted.
	var noStore int64
	if err := s.db.WithContext(ctx).
		Table("employees e").
		Joins("JOIN stores s ON s.id = e.tenant_id").
		Where("e.deleted_at IS NULL AND s.deleted_at IS NOT NULL").
		Count(&noStore).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: noStore count")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// noRole and expiredTokens share one scan — separate struct to avoid clobbering noStore.
	var roleAndExpiry struct {
		NoRole        int64 `gorm:"column:no_role"`
		ExpiredTokens int64 `gorm:"column:expired_tokens"`
	}
	if err := s.db.WithContext(ctx).
		Table("employees").
		Where("deleted_at IS NULL").
		Select(`
			COUNT(*) FILTER (WHERE job_role = '' OR job_role IS NULL)          AS no_role,
			COUNT(*) FILTER (WHERE claim_token IS NOT NULL AND created_at < ?) AS expired_tokens
		`, expiryDate).
		Scan(&roleAndExpiry).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: integrity counts")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// ── 4. Activity ───────────────────────────────────────────────────────────

	// Recent stores (last 5).
	type storeRow struct {
		ID        string    `gorm:"column:id"`
		Name      string    `gorm:"column:name"`
		CreatedAt time.Time `gorm:"column:created_at"`
	}
	var storeRows []storeRow
	if err := s.db.WithContext(ctx).
		Table("stores").
		Select("id, name, created_at").
		Where("deleted_at IS NULL").
		Order("created_at DESC").
		Limit(5).
		Scan(&storeRows).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: recent stores")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// Recent invites (last 5 employees invited by email, bound or not).
	type inviteRow struct {
		EmployeeID string    `gorm:"column:id"`
		Email      string    `gorm:"column:email"`
		CreatedAt  time.Time `gorm:"column:created_at"`
	}
	var inviteRows []inviteRow
	if err := s.db.WithContext(ctx).
		Table("employees").
		Select("id, email, created_at").
		Where("deleted_at IS NULL AND email <> ''").
		Order("created_at DESC").
		Limit(5).
		Scan(&inviteRows).Error; err != nil {
		s.logger.WithError(err).Error("dashboard: recent invites")
		return nil, apierror.Internal("failed to compute dashboard").WithKey("errors.unknown")
	}

	// ── Assemble ──────────────────────────────────────────────────────────────

	recentStores := make([]RecentStore, len(storeRows))
	for i, r := range storeRows {
		recentStores[i] = RecentStore{
			ID:        r.ID,
			Name:      r.Name,
			CreatedAt: r.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	recentInvites := make([]RecentInvite, len(inviteRows))
	for i, r := range inviteRows {
		recentInvites[i] = RecentInvite{
			EmployeeID: r.EmployeeID,
			Email:      r.Email,
			CreatedAt:  r.CreatedAt.UTC().Format(time.RFC3339),
		}
	}

	return &AdminDashboard{
		Organization: DashboardOrganization{
			TotalStores:    totalStores,
			ActiveStores:   activeStores,
			InactiveStores: totalStores - activeStores,
		},
		Users: DashboardUsers{
			TotalEmployees:  ec.Total,
			TotalManagers:   ec.Managers,
			ActiveEmployees: ec.Active,
			PendingInvites:  ec.Pending,
			Unclaimed:       ec.Unclaim,
		},
		Integrity: DashboardIntegrity{
			NoStore:       noStore,
			NoRole:        roleAndExpiry.NoRole,
			ExpiredTokens: roleAndExpiry.ExpiredTokens,
		},
		Activity: DashboardActivity{
			RecentStores:  recentStores,
			RecentInvites: recentInvites,
		},
	}, nil
}
