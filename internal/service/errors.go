package service

import (
	"errors"

	"github.com/ovander/backendkit/apierror"
	"github.com/ovander/parashift/internal/repo"
)

// optimisticErr maps a repository optimistic-lock conflict (ARC-3) to a typed
// 409 Conflict AppError so the client can reload and retry; for any other error
// it returns the provided fallback (typically a generic 500). This keeps the
// concurrency signal from being swallowed when services translate repo errors.
func optimisticErr(err error, fallback *apierror.AppError) error {
	if errors.Is(err, repo.ErrOptimisticLock) {
		return apierror.Conflict("the record was modified by someone else; please reload and try again").
			WithKey("errors.conflict")
	}
	return fallback
}
