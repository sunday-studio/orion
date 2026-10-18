package db

import "time"

// InstanceSettings is the single row (id 1) of settings an operator edits in the Console.
type InstanceSettings struct {
	ID           int    `gorm:"primaryKey"`
	InstanceName string `gorm:"not null;default:''"`
	// PublicURL, when set, replaces ORION_PUBLIC_URL.
	PublicURL string `gorm:"not null;default:''"`
	Timezone  string `gorm:"not null;default:UTC"`
	// TimeFormat is "12" or "24".
	TimeFormat string `gorm:"not null;default:24"`
	// RegistrationToken, when set, replaces ORION_ENROLLMENT_TOKEN as the secret agents register with.
	RegistrationToken          string `gorm:"not null;default:''"`
	RegistrationTokenRotatedAt *time.Time
	UpdatedBy                  string `gorm:"not null;default:''"`
	UpdatedAt                  time.Time
}

// APIToken is a credential for scripts and integrations; only its SHA-256 is stored.
type APIToken struct {
	ID        string `gorm:"primaryKey"`
	Name      string `gorm:"not null"`
	Prefix    string `gorm:"not null"`
	TokenHash string `gorm:"not null;uniqueIndex:idx_api_tokens_hash"`
	// Scope is read (GET only) or write (the whole admin API except token management).
	Scope      string `gorm:"not null;default:read"`
	CreatedBy  string `gorm:"not null;default:''"`
	CreatedAt  time.Time
	LastUsedAt *time.Time
	RevokedAt  *time.Time
}
