package service

import (
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"

	"gorm.io/gorm"
)

type AlertService struct {
	db       *gorm.DB
	logger   *logging.Logger
	channels []config.AlertChannelConfig
}

func NewAlertService(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *AlertService {
	var channels []config.AlertChannelConfig
	if cfg != nil {
		channels = cfg.AlertChannels
	}
	return &AlertService{
		db:       database,
		logger:   logger,
		channels: channels,
	}
}

func (s *AlertService) QueueIncidentNotifications(incidentID string, eventType string) error {
	if len(s.channels) == 0 {
		return s.createDelivery(db.AlertDelivery{
			IncidentID: incidentID,
			EventType:  eventType,
			Channel:    "none",
			Type:       "none",
			Status:     "suppressed",
			Error:      "no alert channels configured",
		})
	}

	for _, channel := range s.channels {
		delivery := db.AlertDelivery{
			IncidentID: incidentID,
			EventType:  eventType,
			Channel:    channel.Name,
			Type:       channel.Type,
			Status:     "pending",
		}
		if !channel.Enabled {
			delivery.Status = "suppressed"
			delivery.Error = "alert channel disabled"
		}
		if err := s.createDelivery(delivery); err != nil {
			return err
		}
	}

	return nil
}

func (s *AlertService) createDelivery(delivery db.AlertDelivery) error {
	delivery.ID = utils.GenerateID("alert_delivery")
	if err := s.db.Create(&delivery).Error; err != nil {
		s.logger.Error("Failed to create alert delivery", "incident_id", delivery.IncidentID, "event_type", delivery.EventType, "error", err)
		return err
	}
	return nil
}
