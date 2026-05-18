package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/smtp"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

type AlertService struct {
	db         *gorm.DB
	logger     *logging.Logger
	cfg        *config.Config
	httpClient *http.Client
}

func NewAlertService(database *gorm.DB, logger *logging.Logger, cfg *config.Config) *AlertService {
	return &AlertService{
		db:         database,
		logger:     logger,
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *AlertService) QueueIncidentNotifications(incidentID string, eventType string) error {
	channels, err := s.deliveryChannels()
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		_, err := s.createDelivery(db.AlertDelivery{
			IncidentID: incidentID,
			EventType:  eventType,
			Channel:    "none",
			Type:       "none",
			Status:     "suppressed",
			Error:      "no alert channels configured",
		})
		return err
	}

	for _, channel := range channels {
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
		createdDelivery, err := s.createDelivery(delivery)
		if err != nil {
			return err
		}
		if createdDelivery.Status != "pending" {
			continue
		}
		if s.inCooldown(incidentID, channel.Name, eventType, createdDelivery.ID) {
			if err := s.updateDelivery(createdDelivery.ID, "cooldown", "alert cooldown active"); err != nil {
				return err
			}
			continue
		}
		if err := s.deliver(channel, incidentID, eventType); err != nil {
			if updateErr := s.updateDelivery(createdDelivery.ID, "failed", err.Error()); updateErr != nil {
				return updateErr
			}
			s.logger.Error("Alert delivery failed", "incident_id", incidentID, "channel", channel.Name, "error", err)
			continue
		}
		if err := s.updateDelivery(createdDelivery.ID, "sent", ""); err != nil {
			return err
		}
	}

	return nil
}

func (s *AlertService) deliveryChannels() ([]db.AlertChannel, error) {
	var channels []db.AlertChannel
	if err := s.db.Order("name ASC").Find(&channels).Error; err != nil {
		s.logger.Error("Failed to load alert channels", "error", err)
		return nil, err
	}
	return channels, nil
}

func (s *AlertService) createDelivery(delivery db.AlertDelivery) (*db.AlertDelivery, error) {
	delivery.ID = utils.GenerateID("alert_delivery")
	if err := s.db.Create(&delivery).Error; err != nil {
		s.logger.Error("Failed to create alert delivery", "incident_id", delivery.IncidentID, "event_type", delivery.EventType, "error", err)
		return nil, err
	}
	return &delivery, nil
}

func (s *AlertService) updateDelivery(deliveryID string, status string, message string) error {
	return s.db.Model(&db.AlertDelivery{}).Where("id = ?", deliveryID).Updates(map[string]interface{}{
		"status": status,
		"error":  message,
	}).Error
}

func (s *AlertService) inCooldown(incidentID string, channelName string, eventType string, currentDeliveryID string) bool {
	if s.cfg == nil || s.cfg.AlertCooldownSeconds <= 0 {
		return false
	}
	since := time.Now().UTC().Add(-time.Duration(s.cfg.AlertCooldownSeconds) * time.Second)

	var count int64
	if err := s.db.Model(&db.AlertDelivery{}).
		Where("incident_id = ? AND channel = ? AND event_type = ? AND id <> ? AND status = ? AND created_at >= ?", incidentID, channelName, eventType, currentDeliveryID, "sent", since).
		Count(&count).Error; err != nil {
		s.logger.Error("Failed to check alert cooldown", "incident_id", incidentID, "channel", channelName, "error", err)
		return false
	}
	return count > 0
}

func (s *AlertService) deliver(channel db.AlertChannel, incidentID string, eventType string) error {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return err
	}

	switch channel.Type {
	case "webhook":
		return s.deliverWebhook(channel, incident, eventType)
	case "email":
		return s.deliverEmail(channel, incident, eventType)
	default:
		return fmt.Errorf("unsupported alert channel type: %s", channel.Type)
	}
}

func (s *AlertService) deliverWebhook(channel db.AlertChannel, incident db.Incident, eventType string) error {
	if channel.WebhookURL == "" {
		return fmt.Errorf("webhook URL is not configured")
	}
	body, err := json.Marshal(map[string]interface{}{
		"event_type": eventType,
		"incident":   incident,
	})
	if err != nil {
		return err
	}

	resp, err := s.httpClient.Post(channel.WebhookURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("webhook returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *AlertService) deliverEmail(channel db.AlertChannel, incident db.Incident, eventType string) error {
	address := fmt.Sprintf("%s:%d", channel.SMTPHost, channel.SMTPPort)
	subject := fmt.Sprintf("Orion alert: %s", incident.Title)
	body := fmt.Sprintf("Event: %s\nIncident: %s\nStatus: %s\nSeverity: %s\nLatest event: %s\n", eventType, incident.ID, incident.Status, incident.Severity, incident.LatestEvent)
	message := []byte(fmt.Sprintf("To: %s\r\nFrom: %s\r\nSubject: %s\r\n\r\n%s", channel.EmailTo, channel.EmailFrom, subject, body))

	var auth smtp.Auth
	if channel.SMTPUsername != "" || channel.SMTPPassword != "" {
		auth = smtp.PlainAuth("", channel.SMTPUsername, channel.SMTPPassword, channel.SMTPHost)
	}
	return smtp.SendMail(address, auth, channel.EmailFrom, []string{channel.EmailTo}, message)
}
