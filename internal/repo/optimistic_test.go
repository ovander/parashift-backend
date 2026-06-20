package repo_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/ovander/parashift/internal/model"
	"github.com/ovander/parashift/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ARC-3: success path — one row affected, version incremented in memory.
func TestOptimisticUpdate_Success(t *testing.T) {
	b, mock := newMockBundle(t)
	emp := &model.Employee{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: uuid.New()}, Versioned: model.Versioned{Version: 3},
		Name: "Alice",
	}

	mock.MatchExpectationsInOrder(true)
	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "employees" SET`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	err := b.Employee.Update(context.Background(), emp)
	require.NoError(t, err)
	assert.Equal(t, 4, emp.Version, "version must be bumped on success")
	assert.NoError(t, mock.ExpectationsWereMet())
}

// ARC-3: zero rows affected (concurrent writer advanced the version, or the row is
// gone) → ErrOptimisticLock, and the in-memory version is rolled back.
func TestOptimisticUpdate_ConflictReturnsTypedError(t *testing.T) {
	b, mock := newMockBundle(t)
	emp := &model.Employee{
		TenantScoped: model.TenantScoped{ID: uuid.New(), TenantID: uuid.New()}, Versioned: model.Versioned{Version: 3},
		Name: "Alice",
	}

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "employees" SET`).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	err := b.Employee.Update(context.Background(), emp)
	require.ErrorIs(t, err, repo.ErrOptimisticLock)
	assert.Equal(t, 3, emp.Version, "version must be rolled back on conflict")
	assert.NoError(t, mock.ExpectationsWereMet())
}
