package model

import (
	"github.com/google/uuid"
	"time"
)

// PendingUser holds the auth_id of a Socrate user who has logged in but has not yet
// been provisioned with an Employee record by an admin.
// The record is upserted on each /me call and deleted when the user is activated.
type PendingUser struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	AuthID    string    `gorm:"not null;uniqueIndex"`
	CreatedAt time.Time `gorm:"not null"`
}
