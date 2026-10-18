package state

import (
	"errors"
	"time"

	"gorm.io/gorm"
)

type logOffsetRecord struct {
	Key       string `gorm:"primaryKey;column:key"`
	FileID    string `gorm:"column:file_id"`
	Offset    int64  `gorm:"column:offset_bytes"`
	Cursor    string `gorm:"column:cursor"`
	UpdatedAt time.Time
}

func (logOffsetRecord) TableName() string { return "log_offsets" }

// LogOffset is a resumable read position for one log source.
type LogOffset struct {
	FileID string
	Offset int64
	Cursor string // journald cursor or container log timestamp
}

// GetLogOffset returns the stored position for key, if any.
func (s *Store) GetLogOffset(key string) (LogOffset, bool, error) {
	var r logOffsetRecord
	err := s.db.Where("key = ?", key).Take(&r).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return LogOffset{}, false, nil
	}
	if err != nil {
		return LogOffset{}, false, err
	}
	return LogOffset{FileID: r.FileID, Offset: r.Offset, Cursor: r.Cursor}, true, nil
}

// SetLogOffset upserts the position for key.
func (s *Store) SetLogOffset(key string, o LogOffset) error {
	r := logOffsetRecord{Key: key, FileID: o.FileID, Offset: o.Offset, Cursor: o.Cursor}
	return s.db.Save(&r).Error
}
