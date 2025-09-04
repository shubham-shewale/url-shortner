package storage

import (
	"time"

	"github.com/google/uuid"
)

type Link struct {
	Code         string     `json:"code" db:"code"`
	LongURL      string     `json:"long_url" db:"long_url"`
	Alias        *string    `json:"alias,omitempty" db:"alias"`
	PasswordHash *string    `json:"-" db:"password_hash"`
	ExpiresAt    *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	MaxClicks    *int       `json:"max_clicks,omitempty" db:"max_clicks"`
	ClickCount   int        `json:"click_count" db:"click_count"`
	CreatedAt    time.Time  `json:"created_at" db:"created_at"`
	OwnerID      *uuid.UUID `json:"owner_id,omitempty" db:"owner_id"`
}
