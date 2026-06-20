package model

import "time"

// TokenRevocation records a per-subject revocation floor used to invalidate
// access tokens before their natural expiry (instant logout / password-change /
// admin-revoke). An access token is rejected when its issued-at (iat) is at or
// before RevokedAfter; re-authenticating yields a token with a later iat that
// passes again.
//
// It is keyed by the Socrate subject (sub) rather than the ParaShift employee ID,
// so it works for any authenticated principal — including platform admins, who
// have no employee record.
type TokenRevocation struct {
	// Sub is the Socrate subject claim (the stable user identifier).
	Sub string `gorm:"primaryKey"`
	// RevokedAfter is the instant from which previously-issued tokens are invalid.
	RevokedAfter time.Time `gorm:"not null"`
	// UpdatedAt is when this floor was last raised.
	UpdatedAt time.Time
}

// TableName pins the table name regardless of GORM's pluralisation rules.
func (TokenRevocation) TableName() string { return "token_revocations" }
