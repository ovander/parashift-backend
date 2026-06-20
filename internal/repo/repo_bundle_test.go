package repo_test

import (
	"context"
	"errors"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/ovander/parashift/internal/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// newMockBundle wires a RepoBundle onto a sqlmock-backed *gorm.DB so we can
// assert transaction control flow (BEGIN/COMMIT/ROLLBACK) without a real DB.
func newMockBundle(t *testing.T) (*repo.RepoBundle, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB}), &gorm.Config{})
	require.NoError(t, err)
	return repo.NewRepoBundle(gdb), mock
}

// DAT-1: a successful fn commits the transaction.
func TestWithTx_CommitsOnSuccess(t *testing.T) {
	b, mock := newMockBundle(t)
	mock.ExpectBegin()
	mock.ExpectCommit()

	called := false
	err := b.WithTx(context.Background(), func(tx *repo.RepoBundle) error {
		called = true
		assert.NotNil(t, tx.ShiftInstance, "tx bundle must expose repositories")
		assert.NotSame(t, b, tx, "tx bundle must be a distinct, tx-bound bundle")
		return nil
	})
	require.NoError(t, err)
	assert.True(t, called)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DAT-1: an error from fn rolls the transaction back and is propagated.
func TestWithTx_RollsBackOnError(t *testing.T) {
	b, mock := newMockBundle(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	sentinel := errors.New("boom")
	err := b.WithTx(context.Background(), func(_ *repo.RepoBundle) error {
		return sentinel
	})
	require.ErrorIs(t, err, sentinel)
	assert.NoError(t, mock.ExpectationsWereMet())
}

// DAT-1: a panic inside fn also rolls back (gorm recovers and re-panics).
func TestWithTx_RollsBackOnPanic(t *testing.T) {
	b, mock := newMockBundle(t)
	mock.ExpectBegin()
	mock.ExpectRollback()

	assert.Panics(t, func() {
		_ = b.WithTx(context.Background(), func(_ *repo.RepoBundle) error {
			panic("kaboom")
		})
	})
	assert.NoError(t, mock.ExpectationsWereMet())
}
