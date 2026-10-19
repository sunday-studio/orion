package service

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	EnrollmentTokenPrefix = "orn_enr_"
	EnrollmentTTL         = 15 * time.Minute
)

var ErrEnrollmentNotFound = errors.New("enrollment not found")

// EnrollmentView is the wizard's picture of one session. Status is derived on read:
// waiting (no machine yet) -> registered (agent exists) -> reporting (first report seen),
// or expired / cancelled.
type EnrollmentView struct {
	ID            string
	Platform      string
	Status        string
	ExpiresAt     time.Time
	CreatedAt     time.Time
	ClaimedAt     *time.Time
	AgentID       string
	AgentName     string
	AgentOS       string
	AgentArch     string
	AgentVersion  string // from the first report
	FirstReportAt *time.Time
}

type EnrollmentService struct {
	db     *gorm.DB
	logger *utils.Logger
}

func NewEnrollmentService(database *gorm.DB, logger *utils.Logger) *EnrollmentService {
	return &EnrollmentService{db: database, logger: logger}
}

func hashEnrollmentToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// IsEnrollmentToken reports whether the presented value looks like a wizard session token.
func IsEnrollmentToken(presented string) bool {
	return strings.HasPrefix(presented, EnrollmentTokenPrefix)
}

// Validate reports whether token names a live Add server session without
// claiming it. Setup uses this before discovery so a typo or expired token is
// rejected before the user spends time choosing monitors.
func (s *EnrollmentService) Validate(token string, now time.Time) bool {
	var session db.EnrollmentSession
	if err := s.db.Where("token_hash = ?", hashEnrollmentToken(token)).First(&session).Error; err != nil {
		return false
	}
	return session.CancelledAt == nil && now.Before(session.ExpiresAt)
}

// Create makes a session and returns the plaintext token, which is never retrievable again.
func (s *EnrollmentService) Create(platform string, now time.Time) (db.EnrollmentSession, string, error) {
	var raw [24]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return db.EnrollmentSession{}, "", err
	}
	token := EnrollmentTokenPrefix + hex.EncodeToString(raw[:])
	session := db.EnrollmentSession{ID: utils.GenerateID("enrollment"), TokenHash: hashEnrollmentToken(token), Platform: platform, ExpiresAt: now.Add(EnrollmentTTL), CreatedAt: now}
	if err := s.db.Create(&session).Error; err != nil {
		return db.EnrollmentSession{}, "", err
	}
	return session, token, nil
}

// Authorize validates a presented session token for the registering machine and binds the
// machine on first use. It returns the session id on success, or "" when the token is not
// a usable session token (unknown, expired, cancelled or bound to another machine).
func (s *EnrollmentService) Authorize(token, machineID string, now time.Time) string {
	var session db.EnrollmentSession
	if err := s.db.Where("token_hash = ?", hashEnrollmentToken(token)).First(&session).Error; err != nil {
		return ""
	}
	if session.CancelledAt != nil || !now.Before(session.ExpiresAt) {
		return ""
	}
	if session.MachineID == machineID && machineID != "" {
		return session.ID
	}
	if session.MachineID != "" {
		return ""
	}
	res := s.db.Model(&db.EnrollmentSession{}).Where("id = ? AND machine_id = ''", session.ID).
		Updates(map[string]any{"machine_id": machineID, "claimed_at": now})
	if res.Error != nil || res.RowsAffected != 1 {
		return ""
	}
	return session.ID
}

// LinkAgent records which agent a session produced.
func (s *EnrollmentService) LinkAgent(sessionID, agentID string) {
	if err := s.db.Model(&db.EnrollmentSession{}).Where("id = ?", sessionID).Update("agent_id", agentID).Error; err != nil {
		s.logger.Error("Failed to link enrollment to agent", "enrollment_id", sessionID, "error", err)
	}
}

func (s *EnrollmentService) Cancel(id string, now time.Time) error {
	res := s.db.Model(&db.EnrollmentSession{}).Where("id = ? AND cancelled_at IS NULL", id).Update("cancelled_at", now)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var n int64
		s.db.Model(&db.EnrollmentSession{}).Where("id = ?", id).Count(&n)
		if n == 0 {
			return ErrEnrollmentNotFound
		}
	}
	return nil
}

func (s *EnrollmentService) Get(id string, now time.Time) (*EnrollmentView, error) {
	var session db.EnrollmentSession
	if err := s.db.Where("id = ?", id).First(&session).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrEnrollmentNotFound
		}
		return nil, err
	}
	return s.view(session, now), nil
}

func (s *EnrollmentService) view(session db.EnrollmentSession, now time.Time) *EnrollmentView {
	v := &EnrollmentView{ID: session.ID, Platform: session.Platform, ExpiresAt: session.ExpiresAt, CreatedAt: session.CreatedAt, ClaimedAt: session.ClaimedAt, AgentID: session.AgentID, Status: "waiting"}
	if session.AgentID != "" {
		var agent db.Agent
		if err := s.db.Where("id = ?", session.AgentID).First(&agent).Error; err == nil {
			v.AgentName, v.AgentOS, v.AgentArch = agent.Name, agent.OS, agent.Arch
		}
		v.Status = "registered"
		var first db.AgentReport
		if err := s.db.Where("agent_id = ?", session.AgentID).Order("created_at ASC").Limit(1).Find(&first).Error; err == nil && first.ID != "" {
			t := first.CreatedAt
			v.FirstReportAt = &t
			v.AgentVersion = first.AgentVersion
			v.Status = "reporting"
		}
		return v
	}
	switch {
	case session.CancelledAt != nil:
		v.Status = "cancelled"
	case !now.Before(session.ExpiresAt):
		v.Status = "expired"
	case session.MachineID != "":
		v.Status = "claimed" // token accepted, registration still in flight
	}
	return v
}
