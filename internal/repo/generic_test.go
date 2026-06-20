package repo_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARC-2: GetByID from the generic base returns (nil, nil) when no row matches,
// scoping by tenant_id + id.
func TestGenericRepo_GetByID_NotFoundReturnsNil(t *testing.T) {
	b, mock := newMockBundle(t)
	tenantID, id := uuid.New(), uuid.New()

	mock.ExpectQuery(`SELECT \* FROM "rules"`).
		WithArgs(tenantID, id, sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"id"}))

	got, err := b.Rule.GetByID(context.Background(), tenantID, id)
	require.NoError(t, err)
	assert.Nil(t, got, "missing row must yield (nil, nil)")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ARC-2: Create from the generic base issues an INSERT.
func TestGenericRepo_Create(t *testing.T) {
	b, mock := newMockBundle(t)

	mock.ExpectBegin()
	mock.ExpectQuery(`INSERT INTO "rules"`).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(uuid.New()))
	mock.ExpectCommit()

	err := b.Rule.Create(context.Background(), &model.Rule{
		TenantScoped: model.TenantScoped{TenantID: uuid.New()},
	})
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ARC-2: Delete from the generic base issues a (soft) delete scoped to tenant + id.
func TestGenericRepo_Delete(t *testing.T) {
	b, mock := newMockBundle(t)
	tenantID, id := uuid.New(), uuid.New()

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "rules" SET "deleted_at"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := b.Rule.Delete(context.Background(), tenantID, id)
	require.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}
