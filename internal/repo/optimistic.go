package repo

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ErrOptimisticLock is returned by versioned updates when the row was modified
// concurrently (the stored version no longer matches the caller's version) or no
// longer exists. Callers map it to a 409 Conflict so the client can refetch and
// retry instead of silently losing an update (ARC-3).
var ErrOptimisticLock = errors.New("optimistic lock conflict: row was modified concurrently")

// versioned is satisfied by any model embedding model.TenantScoped.
type versioned interface {
	GetID() uuid.UUID
	GetVersion() int
	SetVersion(int)
}

// updateOptimistic performs a full-row update guarded by the optimistic-lock
// version. It writes every column (preserving the prior Save() semantics) except
// immutable bookkeeping fields, scoping the WHERE clause to the primary key and
// the prior version and bumping the version atomically. When no row matches —
// because another writer advanced the version, or the row was deleted — it
// returns ErrOptimisticLock and leaves the entity's in-memory version unchanged.
func updateOptimistic(db *gorm.DB, ctx context.Context, entity versioned) error {
	prev := entity.GetVersion()
	entity.SetVersion(prev + 1)

	res := db.WithContext(ctx).
		Model(entity).
		Select("*").
		Omit("id", "created_at", "deleted_at").
		Where("version = ?", prev).
		Updates(entity)

	if res.Error != nil {
		entity.SetVersion(prev) // undo the in-memory bump on failure
		return res.Error
	}
	if res.RowsAffected == 0 {
		entity.SetVersion(prev)
		return ErrOptimisticLock
	}
	return nil
}
