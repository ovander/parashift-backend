package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/backendkit/ctxutil"
	"github.com/ovander/backendkit/socrate"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/event"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/sirupsen/logrus"
)

// EmployeeService manages employees within a store (tenant).
type EmployeeService struct {
	repo      repo.EmployeeRepository
	contract  repo.ContractRepository
	emitter   *event.Emitter
	logger    *logrus.Entry
	socrate   *socrate.Client // nil in tests / when Socrate is not configured
}

// NewEmployeeService creates a new EmployeeService.
func NewEmployeeService(repo repo.EmployeeRepository, contract repo.ContractRepository, emitter *event.Emitter, logger *logrus.Entry) *EmployeeService {
	return &EmployeeService{
		repo:     repo,
		contract: contract,
		emitter:  emitter,
		logger:   logger,
	}
}

// WithSocrateClient attaches a Socrate API client used for sending invite emails.
func (s *EmployeeService) WithSocrateClient(c *socrate.Client) *EmployeeService {
	s.socrate = c
	return s
}

// GetByID retrieves an employee by tenant and ID. Implements EmployeeLookup interface.
func (s *EmployeeService) GetByID(ctx context.Context, tenantID, id uuid.UUID) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)
	emp, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get employee")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", id.String()).WithKey("errors.unknown")
	}
	return emp, nil
}

// GetByAuthID retrieves an employee by their authentication ID (global lookup).
func (s *EmployeeService) GetByAuthID(ctx context.Context, authID string) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)
	emp, err := s.repo.GetByAuthID(ctx, authID)
	if err != nil {
		logger.WithError(err).Error("failed to get employee by auth id")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", authID).WithKey("errors.unknown")
	}
	return emp, nil
}

// ListByStore retrieves all employees for a tenant. Implements EmployeeLookup interface.
func (s *EmployeeService) ListByStore(ctx context.Context, tenantID uuid.UUID) ([]*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)

	var employees []*model.Employee
	page := 0
	pageSize := 1000

	for {
		batch, _, err := s.repo.List(ctx, tenantID, page, pageSize)
		if err != nil {
			logger.WithError(err).Error("failed to list employees")
			return nil, apierror.Internal("failed to list employees").WithKey("errors.unknown")
		}
		employees = append(employees, batch...)
		if len(batch) < pageSize {
			break
		}
		page++
	}

	return employees, nil
}

// List retrieves all employees for a tenant with pagination.
func (s *EmployeeService) List(ctx context.Context, tenantID uuid.UUID, page, pageSize int) ([]*model.Employee, int64, error) {
	logger := ctxutil.GetLogger(ctx)
	emps, total, err := s.repo.List(ctx, tenantID, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list employees")
		return nil, 0, apierror.Internal("failed to list employees").WithKey("errors.unknown")
	}
	return emps, total, nil
}

// Create creates a new employee.
//
// Identity binding strategy (in priority order):
//  1. req.Email set  → call Socrate InviteUserAsService; Socrate creates the account,
//     sends the invite email, and returns the new user's numeric ID which is stored as
//     auth_id immediately. No claim token is generated.
//  2. req.AuthID set → bind directly (admin knows the Socrate sub already).
//  3. Neither set    → generate a one-time claim_token as a fallback (manual sharing).
func (s *EmployeeService) Create(ctx context.Context, tenantID uuid.UUID, req dto.CreateEmployeeRequest) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)

	if req.Name == "" {
		return nil, apierror.BadRequest("name is required").WithKey("errors.invalidInput")
	}
	if req.Position != "manager" && req.Position != "employee" {
		return nil, apierror.BadRequest("position must be 'manager' or 'employee'").WithKey("errors.invalidInput")
	}
	// job_role is required for employees but optional for managers (who manage
	// the store rather than filling shift slots requiring a specific role).
	if req.JobRole == "" && req.Position == "employee" {
		return nil, apierror.BadRequest("job_role is required for employees").WithKey("errors.invalidInput")
	}

	emp := &model.Employee{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Name:      req.Name,
		Position:  req.Position,
		JobRole:   req.JobRole,
		Email:     req.Email,
		AuthID:    req.AuthID,
		StartDate: req.StartDate,
	}

	switch {
	case req.Email != "" && s.socrate != nil:
		// Path 1: invite via Socrate — Socrate creates the account and sends the email.
		// Uses InviteUserAsService (POST /api/apps/{id}/service/users) which is the
		// dedicated M2M endpoint that accepts client_credentials tokens (sub: "app:{id}").
		// POST /api/apps/{id}/users is user-JWT-only and rejects service tokens.
		socrateUser, err := s.socrate.InviteUserAsService(ctx, socrate.ServiceInviteRequest{
			Email: req.Email,
			Role:  "user",
		})
		switch {
		case err == nil && socrateUser != nil && socrateUser.UserID != 0:
			// Socrate created / found the account — bind the auth_id immediately.
			emp.AuthID = fmt.Sprintf("%d", socrateUser.UserID)
			// Log a warning when the invite email was not delivered (SMTP misconfiguration,
			// invalid address, etc.) so that admins can investigate and resend if needed.
			if !socrateUser.EmailSent {
				logger.WithField("email_error", socrateUser.EmailError).
					Warn("Socrate user created but invite email was not delivered; admin can resend via /resend-invite")
			}
		case errors.Is(err, socrate.ErrUserAlreadyExists):
			// User already exists in Socrate — they can log in and claim the record.
			// Leave auth_id empty; the existing user will claim via the link or next login.
			token := uuid.New().String()
			emp.ClaimToken = &token
		default:
			// Socrate call failed (admin port unreachable, bad credentials, etc.).
			// Degrade gracefully: the email is stored for future retry; a claim token is
			// generated so the admin can still share a manual invite link in the meantime.
			logger.WithError(err).Warn("Socrate invite failed, falling back to claim token")
			token := uuid.New().String()
			emp.ClaimToken = &token
		}

	case req.AuthID != "":
		// Path 2: admin already knows the sub — bind directly (no Socrate call).

	default:
		// Path 3: no email, no auth_id — generate a fallback claim token.
		token := uuid.New().String()
		emp.ClaimToken = &token
	}

	if err := s.repo.Create(ctx, emp); err != nil {
		logger.WithError(err).Error("failed to create employee")
		return nil, apierror.Internal("failed to create employee").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeEmployeeCreated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  emp,
	})

	return emp, nil
}

// ClaimByToken binds the supplied Socrate sub to the employee identified by the one-time
// claim token. The token is cleared after successful binding.
func (s *EmployeeService) ClaimByToken(ctx context.Context, token, sub string) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)

	if token == "" || sub == "" {
		return nil, apierror.BadRequest("token and sub are required").WithKey("errors.invalidInput")
	}

	emp, err := s.repo.GetByClaimToken(ctx, token)
	if err != nil {
		logger.WithError(err).Error("failed to look up claim token")
		return nil, apierror.Internal("failed to process claim").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("invite", token).WithKey("errors.unknown")
	}

	// Bind the sub and clear the token.
	emp.AuthID = sub
	emp.ClaimToken = nil
	emp.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, emp); err != nil {
		logger.WithError(err).Error("failed to claim employee record")
		return nil, apierror.Internal("failed to claim invite").WithKey("errors.unknown")
	}

	return emp, nil
}

// Update updates an employee.
func (s *EmployeeService) Update(ctx context.Context, tenantID, id uuid.UUID, req dto.UpdateEmployeeRequest) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)

	emp, err := s.repo.GetByID(ctx, tenantID, id)
	if err != nil {
		logger.WithError(err).Error("failed to get employee for update")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", id.String()).WithKey("errors.unknown")
	}

	if req.Name != nil {
		emp.Name = *req.Name
	}
	if req.Position != nil {
		if *req.Position != "manager" && *req.Position != "employee" {
			return nil, apierror.BadRequest("position must be 'manager' or 'employee'").WithKey("errors.invalidInput")
		}
		emp.Position = *req.Position
	}
	if req.JobRole != nil {
		if *req.JobRole == "" {
			return nil, apierror.BadRequest("job_role cannot be empty").WithKey("errors.invalidInput")
		}
		emp.JobRole = *req.JobRole
	}
	if req.Locale != nil {
		if *req.Locale != "fr" && *req.Locale != "en" {
			return nil, apierror.BadRequest("locale must be 'fr' or 'en'").WithKey("errors.invalidInput")
		}
		emp.Locale = *req.Locale
	}

	emp.UpdatedAt = time.Now()
	if err := s.repo.Update(ctx, emp); err != nil {
		logger.WithError(err).Error("failed to update employee")
		return nil, apierror.Internal("failed to update employee").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeEmployeeUpdated,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  emp,
	})

	return emp, nil
}

// Delete deletes an employee.
func (s *EmployeeService) Delete(ctx context.Context, tenantID, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	if err := s.repo.Delete(ctx, tenantID, id); err != nil {
		logger.WithError(err).Error("failed to delete employee")
		return apierror.Internal("failed to delete employee").WithKey("errors.unknown")
	}

	// Publish event
	s.emitter.Publish(event.Event{
		Type:     event.TypeEmployeeDeleted,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": id},
	})

	return nil
}

// ── Admin / cross-tenant methods ───────────────────────────────────────────────

// ListAll retrieves employees across all stores with optional filtering and pagination.
func (s *EmployeeService) ListAll(ctx context.Context, filter repo.EmployeeFilter, page, pageSize int) ([]*model.Employee, int64, error) {
	logger := ctxutil.GetLogger(ctx)
	emps, total, err := s.repo.ListAll(ctx, filter, page, pageSize)
	if err != nil {
		logger.WithError(err).Error("failed to list all employees")
		return nil, 0, apierror.Internal("failed to list employees").WithKey("errors.unknown")
	}
	return emps, total, nil
}

// GetByIDGlobal retrieves any employee by ID regardless of tenant.
func (s *EmployeeService) GetByIDGlobal(ctx context.Context, id uuid.UUID) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)
	emp, err := s.repo.GetByIDGlobal(ctx, id)
	if err != nil {
		logger.WithError(err).Error("failed to get employee globally")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", id.String()).WithKey("errors.unknown")
	}
	return emp, nil
}

// UpdateGlobal updates any employee regardless of tenant (admin-only).
func (s *EmployeeService) UpdateGlobal(ctx context.Context, id uuid.UUID, req dto.UpdateEmployeeRequest) (*model.Employee, error) {
	logger := ctxutil.GetLogger(ctx)

	emp, err := s.repo.GetByIDGlobal(ctx, id)
	if err != nil {
		logger.WithError(err).Error("failed to get employee for global update")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", id.String()).WithKey("errors.unknown")
	}

	if req.Name != nil {
		emp.Name = *req.Name
	}
	if req.Position != nil {
		emp.Position = *req.Position
	}
	if req.JobRole != nil {
		emp.JobRole = *req.JobRole
	}
	if req.StartDate != nil {
		emp.StartDate = *req.StartDate
	}
	emp.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, emp); err != nil {
		logger.WithError(err).Error("failed to update employee globally")
		return nil, apierror.Internal("failed to update employee").WithKey("errors.unknown")
	}

	s.emitter.Publish(event.Event{
		Type:     event.TypeEmployeeUpdated,
		TenantID: emp.TenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  emp,
	})

	return emp, nil
}

// DeleteGlobal soft-deletes any employee regardless of tenant (admin-only).
func (s *EmployeeService) DeleteGlobal(ctx context.Context, id uuid.UUID) error {
	logger := ctxutil.GetLogger(ctx)

	// Fetch employee before delete so we can capture the correct tenant ID for the audit event.
	emp, _ := s.repo.GetByIDGlobal(ctx, id)

	if err := s.repo.DeleteGlobal(ctx, id); err != nil {
		logger.WithError(err).Error("failed to delete employee globally")
		return apierror.Internal("failed to delete employee").WithKey("errors.unknown")
	}

	// Publish event (best-effort: if emp was nil, TenantID will be uuid.Nil)
	tenantID := uuid.Nil
	if emp != nil {
		tenantID = emp.TenantID
	}
	s.emitter.Publish(event.Event{
		Type:     event.TypeEmployeeDeleted,
		TenantID: tenantID,
		UserID:   ctxutil.GetUserID(ctx),
		Payload:  map[string]interface{}{"id": id},
	})

	return nil
}

// ResendInviteResult carries the outcome of a ResendInvite call so the handler
// can surface different UX paths (email sent vs. fallback claim link).
type ResendInviteResult struct {
	EmailSent  bool    `json:"email_sent"`
	ClaimToken *string `json:"claim_token,omitempty"`
}

// ResendInvite resends the invite to a manager or employee.
//
// The strategy mirrors Create:
//   - Employee NOT yet in Socrate (!auth_id): call InviteUserAsService to create
//     the Socrate account and send the invite email. On success, bind the returned
//     auth_id so subsequent resends take the magic-link path.
//   - Employee already in Socrate (auth_id set): send a Socrate magic link.
//   - Socrate not configured: return an error (invite email is required for resend).
func (s *EmployeeService) ResendInvite(ctx context.Context, id uuid.UUID) (*ResendInviteResult, error) {
	logger := ctxutil.GetLogger(ctx)

	if s.socrate == nil {
		return nil, apierror.BadRequest("invite email service is not configured").WithKey("errors.emailServiceUnavailable")
	}

	emp, err := s.repo.GetByIDGlobal(ctx, id)
	if err != nil {
		logger.WithError(err).Error("failed to get employee for invite resend")
		return nil, apierror.Internal("failed to get employee").WithKey("errors.unknown")
	}
	if emp == nil {
		return nil, apierror.NotFound("employee", id.String()).WithKey("errors.unknown")
	}
	if emp.Email == "" {
		return nil, apierror.BadRequest("employee has no email address on record").WithKey("errors.noEmailAddress")
	}

	// ── Path A: employee is not yet in Socrate ────────────────────────────────
	// Use InviteUserAsService (POST /api/apps/{id}/service/users) — same as Create.
	// This is the dedicated M2M endpoint that accepts client_credentials tokens.
	if emp.AuthID == "" {
		socrateUser, inviteErr := s.socrate.InviteUserAsService(ctx, socrate.ServiceInviteRequest{
			Email: emp.Email,
			Role:  "user",
		})

		switch {
		case inviteErr == nil && socrateUser != nil && socrateUser.UserID != 0:
			// Socrate created / found the account — bind auth_id immediately.
			emp.AuthID = fmt.Sprintf("%d", socrateUser.UserID)
			emp.ClaimToken = nil
			emp.UpdatedAt = time.Now()
			if updateErr := s.repo.Update(ctx, emp); updateErr != nil {
				logger.WithError(updateErr).Warn("resend invite: failed to persist auth_id after Socrate invite")
				// Non-fatal: continue and report the email status.
			}
			if !socrateUser.EmailSent {
				logger.WithField("email_error", socrateUser.EmailError).
					Warn("Socrate user created but invite email was not delivered during resend")
			}
			logger.WithField("employee_id", id).Info("resend invite: Socrate user created/found, invite sent")
			return &ResendInviteResult{EmailSent: socrateUser.EmailSent}, nil

		case errors.Is(inviteErr, socrate.ErrUserAlreadyExists):
			// User exists in Socrate but we don't have their ID — fall through to
			// magic link below by returning a claim token as a safe fallback.
			token := uuid.New().String()
			emp.ClaimToken = &token
			emp.UpdatedAt = time.Now()
			_ = s.repo.Update(ctx, emp)
			logger.WithField("employee_id", id).Info("resend invite: Socrate user exists, generated claim token")
			return &ResendInviteResult{EmailSent: false, ClaimToken: &token}, nil

		default:
			// Socrate call failed — generate a fresh claim token as a fallback.
			logger.WithError(inviteErr).Warn("resend invite: Socrate invite failed, falling back to claim token")
			token := uuid.New().String()
			emp.ClaimToken = &token
			emp.UpdatedAt = time.Now()
			_ = s.repo.Update(ctx, emp)
			return &ResendInviteResult{EmailSent: false, ClaimToken: &token}, nil
		}
	}

	// ── Path B: employee is already in Socrate — send a magic link ────────────
	if _, err := s.socrate.SendMagicLink(ctx, emp.Email); err != nil {
		if errors.Is(err, socrate.ErrMagicLinkRateLimited) {
			return nil, apierror.BadRequest("invite email rate limit exceeded, please try again later").WithKey("errors.rateLimited")
		}
		logger.WithError(err).Error("failed to send magic link")
		return nil, apierror.Internal("failed to resend invite").WithKey("errors.unknown")
	}

	logger.WithField("employee_id", id).Info("resend invite: magic link sent")
	return &ResendInviteResult{EmailSent: true}, nil
}
