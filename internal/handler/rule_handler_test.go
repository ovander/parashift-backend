package handler_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/dto"
	"github.com/ovander/parashift/internal/handler"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/service"
	"github.com/ovander/parashift/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test helper ─────────────────────────────────────────────────────────────

func newRuleHandler(repo *testutil.MockRuleRepo) *handler.RuleHandler {
	svc := service.NewRuleService(repo, newTestLogger())
	return handler.NewRuleHandler(svc)
}

func newTestRule(tenantID uuid.UUID) *model.Rule {
	cfg, _ := json.Marshal(model.RoleConfig{RequiredRole: "cashier"})
	return &model.Rule{
		TenantScoped: model.TenantScoped{
			ID:        uuid.New(),
			TenantID:  tenantID,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
		Type:          model.RuleTypeRole,
		Configuration: cfg,
		Severity:      model.RuleSeverityWarning,
		Enabled:       true,
		Description:   "test rule",
	}
}

// ruleReq wraps a request with managerCtx and the storeId chi param.
func ruleReq(method, path string, body any, storeID uuid.UUID) *http.Request {
	var req *http.Request
	if body != nil {
		req = httptest.NewRequest(method, path, jsonBody(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParam(req, "storeId", storeID.String())
	return req
}

// ─── List ─────────────────────────────────────────────────────────────────────

func TestRuleHandler_List_OK(t *testing.T) {
	storeID := uuid.New()
	rule := newTestRule(storeID)
	repo := &testutil.MockRuleRepo{
		ListFn: func(_ context.Context, _ uuid.UUID) ([]*model.Rule, error) {
			return []*model.Rule{rule}, nil
		},
	}
	h := newRuleHandler(repo)

	req := ruleReq(http.MethodGet, "/rules", nil, storeID)
	rr := httptest.NewRecorder()
	h.List(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp []dto.RuleResponse
	require.NoError(t, decodeJSON(rr, &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, rule.ID, resp[0].ID)
}

func TestRuleHandler_List_InvalidStoreID(t *testing.T) {
	h := newRuleHandler(&testutil.MockRuleRepo{})
	req := httptest.NewRequest(http.MethodGet, "/rules", nil)
	req = req.WithContext(managerCtx(uuid.New()))
	req = withChiURLParam(req, "storeId", "not-a-uuid")
	rr := httptest.NewRecorder()
	h.List(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRuleHandler_List_WrongStore(t *testing.T) {
	// tenantID in context differs from storeId URL param → Forbidden
	tenantID := uuid.New()
	differentStore := uuid.New()

	h := newRuleHandler(&testutil.MockRuleRepo{})
	req := httptest.NewRequest(http.MethodGet, "/rules", nil)
	req = req.WithContext(managerCtx(tenantID))
	req = withChiURLParam(req, "storeId", differentStore.String())
	rr := httptest.NewRecorder()
	h.List(rr, req)
	assert.Equal(t, http.StatusForbidden, rr.Code)
}

// ─── Get ──────────────────────────────────────────────────────────────────────

func TestRuleHandler_Get_OK(t *testing.T) {
	storeID := uuid.New()
	rule := newTestRule(storeID)
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return rule, nil
		},
	}
	h := newRuleHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/rules/"+rule.ID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": rule.ID.String()})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.RuleResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, rule.ID, resp.ID)
	assert.Equal(t, rule.Type, resp.RuleType)
	assert.Equal(t, rule.Severity, resp.Severity)
}

func TestRuleHandler_Get_NotFound(t *testing.T) {
	storeID := uuid.New()
	ruleID := uuid.New()
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return nil, nil
		},
	}
	h := newRuleHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/rules/"+ruleID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": ruleID.String()})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRuleHandler_Get_InvalidRuleID(t *testing.T) {
	storeID := uuid.New()
	h := newRuleHandler(&testutil.MockRuleRepo{})

	req := httptest.NewRequest(http.MethodGet, "/rules/bad-id", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": "bad-id"})
	rr := httptest.NewRecorder()
	h.Get(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

// ─── Create ───────────────────────────────────────────────────────────────────

func TestRuleHandler_Create_OK(t *testing.T) {
	storeID := uuid.New()
	repo := &testutil.MockRuleRepo{
		CreateFn: func(_ context.Context, r *model.Rule) error {
			r.ID = uuid.New()
			r.CreatedAt = time.Now()
			r.UpdatedAt = time.Now()
			return nil
		},
	}
	h := newRuleHandler(repo)

	enabled := true
	body := dto.CreateRuleRequest{
		Type:          model.RuleTypeRole,
		Configuration: map[string]interface{}{"required_role": "cashier"},
		Severity:      model.RuleSeverityWarning,
		Enabled:       &enabled,
		Description:   "cashier only",
	}
	req := ruleReq(http.MethodPost, "/rules", body, storeID)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	var resp dto.RuleResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, model.RuleTypeRole, resp.RuleType)
	assert.Equal(t, model.RuleSeverityWarning, resp.Severity)
	assert.True(t, resp.IsEnabled)
}

func TestRuleHandler_Create_InvalidType(t *testing.T) {
	storeID := uuid.New()
	h := newRuleHandler(&testutil.MockRuleRepo{})

	body := dto.CreateRuleRequest{
		Type:          "UNKNOWN_TYPE",
		Configuration: map[string]interface{}{},
		Severity:      model.RuleSeverityWarning,
	}
	req := ruleReq(http.MethodPost, "/rules", body, storeID)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRuleHandler_Create_InvalidSeverity(t *testing.T) {
	storeID := uuid.New()
	h := newRuleHandler(&testutil.MockRuleRepo{})

	body := dto.CreateRuleRequest{
		Type:          model.RuleTypeRole,
		Configuration: map[string]interface{}{"required_role": "cashier"},
		Severity:      "CRITICAL", // invalid
	}
	req := ruleReq(http.MethodPost, "/rules", body, storeID)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestRuleHandler_Create_RepoError(t *testing.T) {
	storeID := uuid.New()
	repo := &testutil.MockRuleRepo{
		CreateFn: func(_ context.Context, _ *model.Rule) error {
			return errors.New("db error")
		},
	}
	h := newRuleHandler(repo)

	enabled := true
	body := dto.CreateRuleRequest{
		Type:          model.RuleTypeRole,
		Configuration: map[string]interface{}{"required_role": "cashier"},
		Severity:      model.RuleSeverityWarning,
		Enabled:       &enabled,
	}
	req := ruleReq(http.MethodPost, "/rules", body, storeID)
	rr := httptest.NewRecorder()
	h.Create(rr, req)

	assert.Equal(t, http.StatusInternalServerError, rr.Code)
}

// ─── Update ───────────────────────────────────────────────────────────────────

func TestRuleHandler_Update_OK(t *testing.T) {
	storeID := uuid.New()
	rule := newTestRule(storeID)
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return rule, nil
		},
		UpdateFn: func(_ context.Context, _ *model.Rule) error {
			return nil
		},
	}
	h := newRuleHandler(repo)

	newSeverity := model.RuleSeverityBlocking
	body := dto.UpdateRuleRequest{Severity: newSeverity}
	req := httptest.NewRequest(http.MethodPut, "/rules/"+rule.ID.String(), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": rule.ID.String()})
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp dto.RuleResponse
	require.NoError(t, decodeJSON(rr, &resp))
	assert.Equal(t, model.RuleSeverityBlocking, resp.Severity)
}

func TestRuleHandler_Update_NotFound(t *testing.T) {
	storeID := uuid.New()
	ruleID := uuid.New()
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return nil, nil
		},
	}
	h := newRuleHandler(repo)

	body := dto.UpdateRuleRequest{Severity: model.RuleSeverityInfo}
	req := httptest.NewRequest(http.MethodPut, "/rules/"+ruleID.String(), jsonBody(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": ruleID.String()})
	rr := httptest.NewRecorder()
	h.Update(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

// ─── Delete ───────────────────────────────────────────────────────────────────

func TestRuleHandler_Delete_OK(t *testing.T) {
	storeID := uuid.New()
	rule := newTestRule(storeID)
	deleted := false
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return rule, nil
		},
		DeleteFn: func(_ context.Context, _, _ uuid.UUID) error {
			deleted = true
			return nil
		},
	}
	h := newRuleHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/rules/"+rule.ID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": rule.ID.String()})
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusNoContent, rr.Code)
	assert.True(t, deleted)
}

func TestRuleHandler_Delete_NotFound(t *testing.T) {
	storeID := uuid.New()
	ruleID := uuid.New()
	repo := &testutil.MockRuleRepo{
		GetByIDFn: func(_ context.Context, _, _ uuid.UUID) (*model.Rule, error) {
			return nil, nil
		},
	}
	h := newRuleHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/rules/"+ruleID.String(), nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": ruleID.String()})
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRuleHandler_Delete_InvalidRuleID(t *testing.T) {
	storeID := uuid.New()
	h := newRuleHandler(&testutil.MockRuleRepo{})

	req := httptest.NewRequest(http.MethodDelete, "/rules/bad", nil)
	req = req.WithContext(managerCtx(storeID))
	req = withChiURLParams(req, map[string]string{"storeId": storeID.String(), "ruleId": "bad"})
	rr := httptest.NewRecorder()
	h.Delete(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
}
