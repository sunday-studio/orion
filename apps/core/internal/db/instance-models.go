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
	// RegistrationToken, when set, replaces ORION_ENROLLMENT_TOKEN as the secret nodes register with.
	RegistrationToken          string `gorm:"not null;default:''"`
	RegistrationTokenRotatedAt *time.Time
	// UpdateCheckEnabled lets Core ask GitHub for the latest release once a day.
	UpdateCheckEnabled bool `gorm:"not null;default:true"`
	// UpdateChannel is stable (releases only) or prerelease (releases and pre-releases).
	UpdateChannel string `gorm:"not null;default:stable"`
	// UpdateLatest* hold what the last successful check found.
	UpdateLatestVersion    string `gorm:"not null;default:''"`
	UpdateLatestReleasedAt *time.Time
	UpdateLatestSecurity   bool   `gorm:"not null;default:false"`
	UpdateLatestURL        string `gorm:"not null;default:''"`
	// UpdateLatestNotes is the first few bullet points of the release notes, one per line.
	UpdateLatestNotes string `gorm:"not null;default:''"`
	UpdateCheckedAt        *time.Time
	// UpdateCheckError is why the last check failed; empty after a good one.
	UpdateCheckError string `gorm:"not null;default:''"`
	// UpdateSkippedVersion is a release the operator chose not to hear about again.
	UpdateSkippedVersion string `gorm:"not null;default:''"`
	UpdatedBy            string `gorm:"not null;default:''"`
	UpdatedAt            time.Time
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
