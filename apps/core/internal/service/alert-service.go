package service

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/smtp"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/logging"
	"orion/core/internal/utils"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultAlertDeliveryMaxAttempts = 3
	defaultAlertDeliveryRetryDelay  = 5 * time.Minute
	alertEventGroupSummary          = "group_summary"
	alertGroupedSummaryPending      = "alert grouped summary pending"
)

type AlertRouteContext struct {
	IncidentID  string `json:"incident_id"`
	EventType   string `json:"event_type"`
	Severity    string `json:"severity"`
	AgentID     string `json:"agent_id"`
	MonitorID   string `json:"monitor_id"`
	MonitorType string `json:"monitor_type"`
}

type AlertRouteDryRunResult struct {
	Event                AlertRouteContext          `json:"event"`
	LegacyFallback       bool                       `json:"legacy_fallback"`
	Suppressed           bool                       `json:"suppressed"`
	SuppressionReason    string                     `json:"suppression_reason,omitempty"`
	RouteEvaluations     []AlertRouteEvaluation     `json:"route_evaluations"`
	DestinationDecisions []AlertDestinationDecision `json:"destination_decisions"`
}

type AlertRouteEvaluation struct {
	Route      db.AlertRoute `json:"route"`
	Matched    bool          `json:"matched"`
	Suppressed bool          `json:"suppressed"`
	Reasons    []string      `json:"reasons"`
}

type AlertDestinationDecision struct {
	RouteID     string `json:"route_id,omitempty"`
	RouteName   string `json:"route_name,omitempty"`
	ChannelID   string `json:"channel_id,omitempty"`
	ChannelName string `json:"channel_name"`
	ChannelType string `json:"channel_type"`
	Status      string `json:"status"`
	Reason      string `json:"reason"`
}

type AlertSMTPServiceTestResult struct {
	SMTPServiceID   string    `json:"smtp_service_id"`
	SMTPServiceName string    `json:"smtp_service_name"`
	Status          string    `json:"status"`
	Stage           string    `json:"stage"`
	Error           string    `json:"error,omitempty"`
	TestedAt        time.Time `json:"tested_at"`
}

type alertDeliveryError struct {
	stage string
	err   error
}

func newAlertDeliveryError(stage string, err error) error {
	return alertDeliveryError{stage: stage, err: err}
}

func (e alertDeliveryError) Error() string {
	if e.err == nil {
		return e.stage
	}
	return e.err.Error()
}

func (e alertDeliveryError) Unwrap() error {
	return e.err
}

func alertDeliveryErrorStage(err error) string {
	var deliveryErr alertDeliveryError
	if errors.As(err, &deliveryErr) && deliveryErr.stage != "" {
		return deliveryErr.stage
	}
	return "transport"
}

type alertGroupingDecision struct {
	GroupID      string
	Policy       string
	Suppress     bool
	SummaryDue   bool
	SummaryDelay time.Duration
	Reason       string
}

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
	event, err := s.LoadAlertRouteContext(incidentID, eventType)
	if err != nil {
		return err
	}
	routes, err := s.alertRoutes()
	if err != nil {
		return err
	}
	groupingPolicy, groupingDelay := s.groupingPolicyForEvent(*event, routes)
	grouping, err := s.groupingDecision(*event, groupingPolicy, groupingDelay)
	if err != nil {
		return err
	}
	if grouping.SummaryDue {
		return s.queueGroupedSummary(*event, grouping, routes)
	}
	if grouping.Suppress {
		return s.queueGroupedSuppression(*event, grouping, routes)
	}
	if len(routes) > 0 {
		return s.queueRouteIncidentNotifications(*event, routes, grouping)
	}
	return s.queueLegacyIncidentNotifications(incidentID, eventType, grouping)
}

func (s *AlertService) queueLegacyIncidentNotifications(incidentID string, eventType string, grouping alertGroupingDecision) error {
	channels, err := s.deliveryChannels()
	if err != nil {
		return err
	}
	if len(channels) == 0 {
		_, err := s.createDelivery(db.AlertDelivery{
			IncidentID:   incidentID,
			AlertGroupID: grouping.GroupID,
			EventType:    eventType,
			Channel:      "none",
			Type:         "none",
			Status:       "suppressed",
			Error:        "no alert channels configured",
		})
		return err
	}

	for _, channel := range channels {
		if !subscribesToAlertEvent(channel, eventType) {
			continue
		}
		delivery := db.AlertDelivery{
			IncidentID:   incidentID,
			AlertGroupID: grouping.GroupID,
			EventType:    eventType,
			Channel:      channel.Name,
			Type:         channel.Type,
			Status:       "pending",
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
		if err := s.attemptDelivery(createdDelivery, func() error {
			return s.deliver(channel, incidentID, eventType)
		}); err != nil {
			s.logger.Error("Alert delivery failed", "incident_id", incidentID, "channel", channel.Name, "error", err)
			continue
		}
	}

	return nil
}

func (s *AlertService) queueRouteIncidentNotifications(event AlertRouteContext, routes []db.AlertRoute, grouping alertGroupingDecision) error {
	plan, err := s.evaluateRoutes(event, routes)
	if err != nil {
		return err
	}

	if plan.Suppressed {
		_, err := s.createDelivery(db.AlertDelivery{
			IncidentID:   event.IncidentID,
			RouteID:      suppressingRouteID(plan),
			AlertGroupID: grouping.GroupID,
			EventType:    event.EventType,
			Channel:      "route",
			Type:         "route",
			Status:       "suppressed",
			Error:        plan.SuppressionReason,
		})
		return err
	}

	if len(plan.DestinationDecisions) == 0 {
		_, err := s.createDelivery(db.AlertDelivery{
			IncidentID:   event.IncidentID,
			AlertGroupID: grouping.GroupID,
			EventType:    event.EventType,
			Channel:      "none",
			Type:         "none",
			Status:       "suppressed",
			Error:        "no alert routes matched",
		})
		return err
	}

	for _, decision := range plan.DestinationDecisions {
		delivery := db.AlertDelivery{
			IncidentID:   event.IncidentID,
			RouteID:      decision.RouteID,
			AlertGroupID: grouping.GroupID,
			EventType:    event.EventType,
			Channel:      decision.ChannelName,
			Type:         decision.ChannelType,
			Status:       decision.Status,
			Error:        decision.Reason,
		}
		if delivery.Status == "" {
			delivery.Status = "pending"
		}
		createdDelivery, err := s.createDelivery(delivery)
		if err != nil {
			return err
		}
		if createdDelivery.Status != "pending" {
			continue
		}

		channel, err := s.alertChannelByID(decision.ChannelID)
		if err != nil {
			if updateErr := s.updateDelivery(createdDelivery.ID, "suppressed", "alert route destination missing"); updateErr != nil {
				return updateErr
			}
			continue
		}
		if s.inCooldown(event.IncidentID, channel.Name, event.EventType, createdDelivery.ID) {
			if err := s.updateDelivery(createdDelivery.ID, "cooldown", "alert cooldown active"); err != nil {
				return err
			}
			continue
		}
		if err := s.attemptDelivery(createdDelivery, func() error {
			return s.deliver(channel, event.IncidentID, event.EventType)
		}); err != nil {
			s.logger.Error("Alert delivery failed", "incident_id", event.IncidentID, "route_id", decision.RouteID, "channel", channel.Name, "error", err)
			continue
		}
	}

	return nil
}

func (s *AlertService) queueGroupedSuppression(event AlertRouteContext, grouping alertGroupingDecision, routes []db.AlertRoute) error {
	delivery := db.AlertDelivery{
		IncidentID:   event.IncidentID,
		AlertGroupID: grouping.GroupID,
		EventType:    event.EventType,
		Channel:      "group",
		Type:         "group",
		Status:       "suppressed",
		Error:        grouping.Reason,
	}
	if len(routes) > 0 {
		delivery.Channel = "route"
		delivery.Type = "route"
	}
	_, err := s.createDelivery(delivery)
	return err
}

func (s *AlertService) queueGroupedSummary(event AlertRouteContext, grouping alertGroupingDecision, routes []db.AlertRoute) error {
	if len(routes) == 0 {
		return s.queueGroupedSuppression(event, alertGroupingDecision{
			GroupID:  grouping.GroupID,
			Suppress: true,
			Reason:   alertGroupedSummaryPending,
		}, routes)
	}

	plan, err := s.evaluateRoutes(event, routes)
	if err != nil {
		return err
	}
	if plan.Suppressed {
		return s.queueGroupedSuppression(event, alertGroupingDecision{
			GroupID:  grouping.GroupID,
			Suppress: true,
			Reason:   plan.SuppressionReason,
		}, routes)
	}

	dueAt := time.Now().UTC().Add(grouping.SummaryDelay)
	scheduled := 0
	for _, decision := range plan.DestinationDecisions {
		if decision.Status != "pending" {
			_, err := s.createDelivery(db.AlertDelivery{
				IncidentID:   event.IncidentID,
				RouteID:      decision.RouteID,
				AlertGroupID: grouping.GroupID,
				EventType:    alertEventGroupSummary,
				Channel:      decision.ChannelName,
				Type:         decision.ChannelType,
				Status:       decision.Status,
				Error:        decision.Reason,
			})
			if err != nil {
				return err
			}
			continue
		}
		if err := s.scheduleGroupedSummaryDelivery(event, grouping.GroupID, decision, dueAt); err != nil {
			return err
		}
		scheduled++
	}

	if scheduled == 0 && len(plan.DestinationDecisions) == 0 {
		_, err := s.createDelivery(db.AlertDelivery{
			IncidentID:   event.IncidentID,
			AlertGroupID: grouping.GroupID,
			EventType:    alertEventGroupSummary,
			Channel:      "none",
			Type:         "none",
			Status:       "suppressed",
			Error:        "no alert routes matched",
		})
		return err
	}
	return nil
}

func (s *AlertService) scheduleGroupedSummaryDelivery(event AlertRouteContext, groupID string, decision AlertDestinationDecision, dueAt time.Time) error {
	var existing db.AlertDelivery
	result := s.db.
		Where("alert_group_id = ? AND route_id = ? AND channel = ? AND type = ? AND event_type = ? AND status = ?", groupID, decision.RouteID, decision.ChannelName, decision.ChannelType, alertEventGroupSummary, "pending").
		Order("created_at DESC").
		Limit(1).
		Find(&existing)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected > 0 {
		return s.db.Model(&db.AlertDelivery{}).Where("id = ?", existing.ID).Updates(map[string]interface{}{
			"incident_id":     event.IncidentID,
			"next_attempt_at": dueAt,
			"error":           alertGroupedSummaryPending,
		}).Error
	}

	_, err := s.createDelivery(db.AlertDelivery{
		IncidentID:    event.IncidentID,
		RouteID:       decision.RouteID,
		AlertGroupID:  groupID,
		EventType:     alertEventGroupSummary,
		Channel:       decision.ChannelName,
		Type:          decision.ChannelType,
		Status:        "pending",
		Error:         alertGroupedSummaryPending,
		MaxAttempts:   defaultAlertDeliveryMaxAttempts,
		NextAttemptAt: &dueAt,
	})
	return err
}

func (s *AlertService) TestChannel(channelID string) (*db.AlertDelivery, error) {
	var channel db.AlertChannel
	if err := s.db.Where("id = ?", channelID).First(&channel).Error; err != nil {
		return nil, err
	}

	delivery, err := s.createDelivery(db.AlertDelivery{
		IncidentID: "alert-channel-test",
		EventType:  "test",
		Channel:    channel.Name,
		Type:       channel.Type,
		Status:     "pending",
	})
	if err != nil {
		return nil, err
	}

	if err := s.attemptDelivery(delivery, func() error {
		return s.deliverTest(channel)
	}); err != nil {
		s.logger.Error("Alert channel test failed", "channel", channel.Name, "error", err)
		return delivery, nil
	}

	return delivery, nil
}

func (s *AlertService) TestSMTPService(smtpServiceID string) (*AlertSMTPServiceTestResult, error) {
	var smtpService db.AlertSMTPService
	if err := s.db.Where("id = ?", smtpServiceID).First(&smtpService).Error; err != nil {
		return nil, err
	}

	result := &AlertSMTPServiceTestResult{
		SMTPServiceID:   smtpService.ID,
		SMTPServiceName: smtpService.Name,
		Status:          "ok",
		Stage:           "connected",
		TestedAt:        time.Now().UTC(),
	}
	if err := s.testSMTPConnectivity(smtpService); err != nil {
		result.Status = "failed"
		result.Stage = alertDeliveryErrorStage(err)
		result.Error = "smtp connectivity failed; check Core logs"
		s.logger.Error("Alert SMTP service test failed", "smtp_service_id", smtpService.ID, "stage", result.Stage, "error", err)
	}
	return result, nil
}

func (s *AlertService) TestEmailDestination(destinationID string) (*db.AlertDelivery, error) {
	var destination db.AlertEmailDestination
	if err := s.db.Where("id = ?", destinationID).First(&destination).Error; err != nil {
		return nil, err
	}

	var smtpService db.AlertSMTPService
	if err := s.db.Where("id = ?", destination.SMTPServiceID).First(&smtpService).Error; err != nil {
		return nil, err
	}

	channel := alertChannelFromEmailDestination(destination, smtpService)
	delivery, err := s.createDelivery(db.AlertDelivery{
		IncidentID: "alert-email-destination-test",
		EventType:  "test",
		Channel:    destination.Name,
		Type:       "email",
		Status:     "pending",
	})
	if err != nil {
		return nil, err
	}

	if err := s.attemptDelivery(delivery, func() error {
		return s.deliverTest(channel)
	}); err != nil {
		s.logger.Error("Alert email destination test failed", "destination", destination.Name, "error", err)
		return delivery, nil
	}

	return delivery, nil
}

func subscribesToAlertEvent(channel db.AlertChannel, eventType string) bool {
	for _, event := range db.DecodeAlertEvents(channel.SubscribedEvents) {
		if event == eventType {
			return true
		}
	}
	return false
}

func (s *AlertService) deliveryChannels() ([]db.AlertChannel, error) {
	var channels []db.AlertChannel
	if err := s.db.Order("name ASC").Find(&channels).Error; err != nil {
		s.logger.Error("Failed to load alert channels", "error", err)
		return nil, err
	}
	destinationChannels, err := s.emailDestinationChannels()
	if err != nil {
		return nil, err
	}
	channels = append(channels, destinationChannels...)
	return channels, nil
}

func (s *AlertService) alertRoutes() ([]db.AlertRoute, error) {
	var routes []db.AlertRoute
	if err := s.db.Order("priority ASC, name ASC").Find(&routes).Error; err != nil {
		s.logger.Error("Failed to load alert routes", "error", err)
		return nil, err
	}
	return routes, nil
}

func (s *AlertService) alertChannelByID(channelID string) (db.AlertChannel, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return db.AlertChannel{}, err
	}
	for _, channel := range channels {
		if channel.ID == channelID {
			return channel, nil
		}
	}
	return db.AlertChannel{}, gorm.ErrRecordNotFound
}

func (s *AlertService) ProcessDueDeliveries(limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}

	now := time.Now().UTC()
	var deliveries []db.AlertDelivery
	if err := s.db.
		Where("status IN ? AND attempt_count < max_attempts AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", []string{"pending", "failed"}, now).
		Order("created_at ASC").
		Limit(limit).
		Find(&deliveries).Error; err != nil {
		return 0, err
	}

	processed := 0
	for i := range deliveries {
		delivery := &deliveries[i]
		channel, err := s.channelForDelivery(*delivery)
		if err != nil {
			if attemptErr := s.attemptDelivery(delivery, func() error {
				return newAlertDeliveryError("channel_lookup", err)
			}); attemptErr != nil {
				s.logger.Error("Alert delivery retry failed", "delivery_id", delivery.ID, "channel", delivery.Channel, "error", attemptErr)
			}
			processed++
			continue
		}
		if err := s.attemptDelivery(delivery, func() error {
			if delivery.EventType == alertEventGroupSummary && delivery.AlertGroupID != "" {
				return s.deliverGroupSummary(channel, delivery.AlertGroupID)
			}
			return s.deliver(channel, delivery.IncidentID, delivery.EventType)
		}); err != nil {
			s.logger.Error("Alert delivery retry failed", "delivery_id", delivery.ID, "channel", delivery.Channel, "error", err)
		}
		processed++
	}
	return processed, nil
}

func (s *AlertService) channelForDelivery(delivery db.AlertDelivery) (db.AlertChannel, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return db.AlertChannel{}, err
	}
	for _, channel := range channels {
		if channel.Name == delivery.Channel && channel.Type == delivery.Type {
			return channel, nil
		}
	}
	return db.AlertChannel{}, gorm.ErrRecordNotFound
}

func (s *AlertService) emailDestinationChannels() ([]db.AlertChannel, error) {
	if !s.db.Migrator().HasTable(&db.AlertEmailDestination{}) {
		return nil, nil
	}

	var destinations []db.AlertEmailDestination
	if err := s.db.Order("name ASC").Find(&destinations).Error; err != nil {
		s.logger.Error("Failed to load alert email destinations", "error", err)
		return nil, err
	}
	if len(destinations) == 0 {
		return nil, nil
	}

	serviceIDs := make([]string, 0, len(destinations))
	seen := map[string]bool{}
	for _, destination := range destinations {
		if destination.SMTPServiceID == "" || seen[destination.SMTPServiceID] {
			continue
		}
		seen[destination.SMTPServiceID] = true
		serviceIDs = append(serviceIDs, destination.SMTPServiceID)
	}
	if len(serviceIDs) == 0 {
		return nil, nil
	}
	if !s.db.Migrator().HasTable(&db.AlertSMTPService{}) {
		return nil, nil
	}

	var smtpServices []db.AlertSMTPService
	if err := s.db.Where("id IN ?", serviceIDs).Find(&smtpServices).Error; err != nil {
		s.logger.Error("Failed to load alert SMTP services", "error", err)
		return nil, err
	}
	servicesByID := make(map[string]db.AlertSMTPService, len(smtpServices))
	for _, smtpService := range smtpServices {
		servicesByID[smtpService.ID] = smtpService
	}

	channels := make([]db.AlertChannel, 0, len(destinations))
	for _, destination := range destinations {
		smtpService, ok := servicesByID[destination.SMTPServiceID]
		if !ok {
			continue
		}
		channels = append(channels, alertChannelFromEmailDestination(destination, smtpService))
	}
	return channels, nil
}

func alertChannelFromEmailDestination(destination db.AlertEmailDestination, smtpService db.AlertSMTPService) db.AlertChannel {
	return db.AlertChannel{
		ID:               destination.ID,
		Name:             destination.Name,
		Type:             "email",
		Enabled:          destination.Enabled && smtpService.Enabled,
		EmailTo:          destination.EmailTo,
		EmailFrom:        smtpService.FromEmail,
		SMTPHost:         smtpService.Host,
		SMTPPort:         smtpService.Port,
		SMTPUsername:     smtpService.Username,
		SMTPPassword:     smtpService.Password,
		SubscribedEvents: destination.SubscribedEvents,
	}
}

func (s *AlertService) createDelivery(delivery db.AlertDelivery) (*db.AlertDelivery, error) {
	delivery.ID = utils.GenerateID("alert_delivery")
	if delivery.Status == "pending" && delivery.MaxAttempts == 0 {
		delivery.MaxAttempts = defaultAlertDeliveryMaxAttempts
	}
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

func (s *AlertService) groupingDecision(event AlertRouteContext, policy string, delay time.Duration) (alertGroupingDecision, error) {
	if policy == db.AlertGroupingPolicyNone {
		return alertGroupingDecision{Policy: policy}, nil
	}
	if event.EventType != db.AlertEventIncidentOpened && event.EventType != db.AlertEventIncidentResolved {
		return alertGroupingDecision{}, nil
	}

	var incident db.Incident
	if err := s.db.Where("id = ?", event.IncidentID).First(&incident).Error; err != nil {
		return alertGroupingDecision{}, err
	}

	switch event.EventType {
	case db.AlertEventIncidentOpened:
		return s.openIncidentGroupingDecision(event, incident, policy, delay)
	case db.AlertEventIncidentResolved:
		return s.resolveIncidentGroupingDecision(event, incident, policy)
	default:
		return alertGroupingDecision{}, nil
	}
}

func (s *AlertService) openIncidentGroupingDecision(event AlertRouteContext, incident db.Incident, policy string, delay time.Duration) (alertGroupingDecision, error) {
	existingGroup, found, err := s.alertGroupForIncident(incident.ID)
	if err != nil {
		return alertGroupingDecision{}, err
	}
	if found {
		return alertGroupingDecision{GroupID: existingGroup.ID, Policy: policy}, nil
	}

	now := time.Now().UTC()
	groupKey := alertGroupKey(event)
	var group db.AlertGroup
	result := s.db.Where("group_key = ? AND status = ?", groupKey, "open").
		Order("last_event_at DESC").
		Limit(1).
		Find(&group)
	if result.Error != nil {
		return alertGroupingDecision{}, result.Error
	}

	if result.RowsAffected == 0 {
		group = db.AlertGroup{
			ID:              utils.GenerateID("alert_group"),
			GroupKey:        groupKey,
			Status:          "open",
			EventType:       event.EventType,
			Severity:        incident.Severity,
			Summary:         alertGroupSummary(event, 1),
			FirstIncidentID: incident.ID,
			LastIncidentID:  incident.ID,
			IncidentCount:   1,
			FirstEventAt:    now,
			LastEventAt:     now,
		}
		if err := s.db.Create(&group).Error; err != nil {
			return alertGroupingDecision{}, err
		}
		if err := s.createAlertGroupMember(group.ID, incident.ID); err != nil {
			return alertGroupingDecision{}, err
		}
		return alertGroupingDecision{GroupID: group.ID, Policy: policy}, nil
	}

	if err := s.createAlertGroupMember(group.ID, incident.ID); err != nil {
		return alertGroupingDecision{}, err
	}
	nextCount := group.IncidentCount + 1
	updates := map[string]interface{}{
		"last_incident_id": incident.ID,
		"incident_count":   nextCount,
		"last_event_at":    now,
		"summary":          alertGroupSummary(event, nextCount),
	}
	if err := s.db.Model(&db.AlertGroup{}).Where("id = ?", group.ID).Updates(updates).Error; err != nil {
		return alertGroupingDecision{}, err
	}

	decision := alertGroupingDecision{
		GroupID: group.ID,
		Policy:  policy,
	}
	if policy == db.AlertGroupingPolicyDelayedSummary {
		decision.SummaryDue = true
		decision.SummaryDelay = delay
		decision.Reason = alertGroupedSummaryPending
		return decision, nil
	}
	decision.Suppress = true
	decision.Reason = "alert grouped into active alert group"
	return decision, nil
}

func (s *AlertService) resolveIncidentGroupingDecision(event AlertRouteContext, incident db.Incident, policy string) (alertGroupingDecision, error) {
	group, found, err := s.alertGroupForIncident(incident.ID)
	if err != nil || !found {
		return alertGroupingDecision{}, err
	}

	now := time.Now().UTC()
	activeCount, err := s.activeAlertGroupIncidentCount(group.ID)
	if err != nil {
		return alertGroupingDecision{}, err
	}
	if activeCount > 0 {
		if err := s.db.Model(&db.AlertGroup{}).Where("id = ?", group.ID).Updates(map[string]interface{}{
			"last_event_at": now,
			"summary":       alertGroupSummary(event, group.IncidentCount),
		}).Error; err != nil {
			return alertGroupingDecision{}, err
		}
		return alertGroupingDecision{
			GroupID:  group.ID,
			Policy:   policy,
			Suppress: true,
			Reason:   "alert grouped; sibling incidents still active",
		}, nil
	}

	if err := s.db.Model(&db.AlertGroup{}).Where("id = ?", group.ID).Updates(map[string]interface{}{
		"status":        "resolved",
		"resolved_at":   &now,
		"last_event_at": now,
		"summary":       fmt.Sprintf("All %d grouped incidents resolved", group.IncidentCount),
	}).Error; err != nil {
		return alertGroupingDecision{}, err
	}
	return alertGroupingDecision{GroupID: group.ID, Policy: policy}, nil
}

func (s *AlertService) alertGroupForIncident(incidentID string) (db.AlertGroup, bool, error) {
	var member db.AlertGroupMember
	result := s.db.Where("incident_id = ?", incidentID).Order("created_at DESC").Limit(1).Find(&member)
	if result.Error != nil {
		return db.AlertGroup{}, false, result.Error
	}
	if result.RowsAffected == 0 {
		return db.AlertGroup{}, false, nil
	}

	var group db.AlertGroup
	if err := s.db.Where("id = ?", member.AlertGroupID).First(&group).Error; err != nil {
		return db.AlertGroup{}, false, err
	}
	return group, true, nil
}

func (s *AlertService) createAlertGroupMember(groupID string, incidentID string) error {
	member := db.AlertGroupMember{
		ID:           utils.GenerateID("alert_group_member"),
		AlertGroupID: groupID,
		IncidentID:   incidentID,
	}
	return s.db.Create(&member).Error
}

func (s *AlertService) activeAlertGroupIncidentCount(groupID string) (int64, error) {
	var count int64
	err := s.db.Table("alert_group_members").
		Joins("JOIN incidents ON incidents.id = alert_group_members.incident_id").
		Where("alert_group_members.alert_group_id = ? AND incidents.status IN ?", groupID, []string{"open", "acknowledged"}).
		Count(&count).Error
	return count, err
}

func (s *AlertService) groupingPolicyForEvent(event AlertRouteContext, routes []db.AlertRoute) (string, time.Duration) {
	if len(routes) == 0 {
		return db.AlertGroupingPolicySuppress, time.Duration(db.DefaultAlertGroupingDelaySeconds) * time.Second
	}
	for _, route := range routes {
		matched, _ := routeMatchesEvent(route, event)
		if !matched || !route.Enabled {
			continue
		}
		return normalizeAlertGroupingPolicy(route.GroupingPolicy), time.Duration(normalizeAlertGroupingDelaySeconds(route.GroupingDelaySeconds)) * time.Second
	}
	return db.AlertGroupingPolicySuppress, time.Duration(db.DefaultAlertGroupingDelaySeconds) * time.Second
}

func normalizeAlertGroupingPolicy(policy string) string {
	switch strings.TrimSpace(policy) {
	case "", db.AlertGroupingPolicySuppress:
		return db.AlertGroupingPolicySuppress
	case db.AlertGroupingPolicyDelayedSummary:
		return db.AlertGroupingPolicyDelayedSummary
	case db.AlertGroupingPolicyNone:
		return db.AlertGroupingPolicyNone
	default:
		return db.AlertGroupingPolicySuppress
	}
}

func normalizeAlertGroupingDelaySeconds(value int) int {
	if value <= 0 {
		return db.DefaultAlertGroupingDelaySeconds
	}
	return value
}

func alertGroupKey(event AlertRouteContext) string {
	monitorType := strings.TrimSpace(event.MonitorType)
	if monitorType == "" {
		monitorType = "unknown"
	}
	return fmt.Sprintf("agent:%s|monitor_type:%s|severity:%s", event.AgentID, monitorType, event.Severity)
}

func alertGroupSummary(event AlertRouteContext, count int) string {
	monitorType := strings.TrimSpace(event.MonitorType)
	if monitorType == "" {
		monitorType = "unknown monitor"
	}
	return fmt.Sprintf("%d %s %s incident(s) on agent %s", count, event.Severity, monitorType, event.AgentID)
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
		return newAlertDeliveryError("load_incident", err)
	}

	switch channel.Type {
	case "webhook":
		return s.deliverWebhook(channel, incident, eventType)
	case "email":
		return s.deliverEmail(channel, incident, eventType)
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
}

func (s *AlertService) deliverGroupSummary(channel db.AlertChannel, groupID string) error {
	payload, err := s.buildAlertGroupSummaryPayload(groupID, time.Now().UTC())
	if err != nil {
		return err
	}
	switch channel.Type {
	case "webhook":
		return s.deliverWebhookPayload(channel, payload)
	case "email":
		return s.deliverEmailPayload(channel, payload)
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
}

func (s *AlertService) buildAlertGroupSummaryPayload(groupID string, deliveredAt time.Time) (AlertPayload, error) {
	var group db.AlertGroup
	if err := s.db.Where("id = ?", groupID).First(&group).Error; err != nil {
		return AlertPayload{}, newAlertDeliveryError("load_alert_group", err)
	}

	incidentID := group.LastIncidentID
	if strings.TrimSpace(incidentID) == "" {
		incidentID = group.FirstIncidentID
	}
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return AlertPayload{}, newAlertDeliveryError("load_incident", err)
	}

	payload := s.buildAlertPayload(incident, alertEventGroupSummary, deliveredAt)
	title := fmt.Sprintf("%d grouped %s incident(s)", group.IncidentCount, group.Severity)
	if strings.TrimSpace(group.Summary) != "" {
		title = group.Summary
	}
	payload.Summary = AlertPayloadSummary{
		Title: title,
		Text:  fmt.Sprintf("%s across alert group %s. Latest incident: %s.", title, group.ID, incident.Title),
	}
	return payload, nil
}

func (s *AlertService) deliverTest(channel db.AlertChannel) error {
	incident := db.Incident{
		ID:          "alert-channel-test",
		Status:      "test",
		Severity:    "info",
		Title:       "Alert channel test",
		LatestEvent: "Manual alert channel test",
	}

	switch channel.Type {
	case "webhook":
		return s.deliverWebhook(channel, incident, "test")
	case "email":
		return s.deliverEmail(channel, incident, "test")
	default:
		return newAlertDeliveryError("channel_type", fmt.Errorf("unsupported alert channel type: %s", channel.Type))
	}
}

func (s *AlertService) deliverWebhook(channel db.AlertChannel, incident db.Incident, eventType string) error {
	payload := s.buildAlertPayload(incident, eventType, time.Now().UTC())
	return s.deliverWebhookPayload(channel, payload)
}

func (s *AlertService) deliverWebhookPayload(channel db.AlertChannel, payload AlertPayload) error {
	if channel.WebhookURL == "" {
		return newAlertDeliveryError("configure", fmt.Errorf("webhook URL is not configured"))
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return newAlertDeliveryError("serialize", err)
	}

	request, err := http.NewRequest(http.MethodPost, channel.WebhookURL, bytes.NewReader(body))
	if err != nil {
		return newAlertDeliveryError("http_request", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("User-Agent", "orion-core-alerts/1")
	request.Header.Set("X-Orion-Payload-Version", AlertPayloadVersion)
	if signingSecret := strings.TrimSpace(channel.WebhookSigningSecret); signingSecret != "" {
		signature := SignAlertWebhookPayload(signingSecret, payload.DeliveredAt, body)
		request.Header.Set(signature.Header, signature.Value)
		request.Header.Set("X-Orion-Timestamp", signature.Timestamp)
	}

	resp, err := s.httpClient.Do(request)
	if err != nil {
		return newAlertDeliveryError("http_request", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return newAlertDeliveryError("http_response", fmt.Errorf("webhook returned status %d", resp.StatusCode))
	}
	return nil
}

func (s *AlertService) deliverEmail(channel db.AlertChannel, incident db.Incident, eventType string) error {
	address := fmt.Sprintf("%s:%d", channel.SMTPHost, channel.SMTPPort)
	payload := s.buildAlertPayload(incident, eventType, time.Now().UTC())
	return s.deliverEmailPayloadAtAddress(channel, payload, address)
}

func (s *AlertService) deliverEmailPayload(channel db.AlertChannel, payload AlertPayload) error {
	address := fmt.Sprintf("%s:%d", channel.SMTPHost, channel.SMTPPort)
	return s.deliverEmailPayloadAtAddress(channel, payload, address)
}

func (s *AlertService) deliverEmailPayloadAtAddress(channel db.AlertChannel, payload AlertPayload, address string) error {
	email := RenderAlertEmail(payload)
	message := alertEmailMessage(channel.EmailTo, channel.EmailFrom, email)

	var auth smtp.Auth
	if channel.SMTPUsername != "" || channel.SMTPPassword != "" {
		auth = smtp.PlainAuth("", channel.SMTPUsername, channel.SMTPPassword, channel.SMTPHost)
	}
	if err := smtp.SendMail(address, auth, channel.EmailFrom, []string{channel.EmailTo}, message); err != nil {
		return newAlertDeliveryError("smtp_send", err)
	}
	return nil
}

func (s *AlertService) testSMTPConnectivity(smtpService db.AlertSMTPService) error {
	address := fmt.Sprintf("%s:%d", smtpService.Host, smtpService.Port)
	dialer := net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.Dial("tcp", address)
	if err != nil {
		return newAlertDeliveryError("smtp_connect", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, smtpService.Host)
	if err != nil {
		return newAlertDeliveryError("smtp_connect", err)
	}
	defer client.Close()

	if err := client.Hello("localhost"); err != nil {
		return newAlertDeliveryError("smtp_hello", err)
	}
	if smtpService.Username != "" || smtpService.Password != "" {
		auth := smtp.PlainAuth("", smtpService.Username, smtpService.Password, smtpService.Host)
		if err := client.Auth(auth); err != nil {
			return newAlertDeliveryError("smtp_auth", err)
		}
	}
	if err := client.Quit(); err != nil {
		return newAlertDeliveryError("smtp_quit", err)
	}
	return nil
}

func alertEmailMessage(to string, from string, email AlertEmailTemplate) []byte {
	const boundary = "orion-alert-boundary-v1"
	message := strings.Builder{}
	message.WriteString("To: " + sanitizeEmailHeader(to) + "\r\n")
	message.WriteString("From: " + sanitizeEmailHeader(from) + "\r\n")
	message.WriteString("Subject: " + email.Subject + "\r\n")
	message.WriteString("MIME-Version: 1.0\r\n")
	message.WriteString("Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n")
	message.WriteString("\r\n")
	message.WriteString("--" + boundary + "\r\n")
	message.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	message.WriteString(email.Body)
	message.WriteString("\r\n--" + boundary + "\r\n")
	message.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	message.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	message.WriteString(email.HTMLBody)
	message.WriteString("\r\n--" + boundary + "--\r\n")
	return []byte(message.String())
}

func (s *AlertService) LoadAlertRouteContext(incidentID string, eventType string) (*AlertRouteContext, error) {
	var incident db.Incident
	if err := s.db.Where("id = ?", incidentID).First(&incident).Error; err != nil {
		return nil, err
	}

	var monitor db.Monitor
	if err := s.db.Where("id = ?", incident.MonitorID).First(&monitor).Error; err != nil {
		monitor = db.Monitor{ID: incident.MonitorID, AgentID: incident.AgentID}
	}

	return &AlertRouteContext{
		IncidentID:  incident.ID,
		EventType:   eventType,
		Severity:    incident.Severity,
		AgentID:     incident.AgentID,
		MonitorID:   incident.MonitorID,
		MonitorType: monitor.Type,
	}, nil
}

func (s *AlertService) DryRunRoutes(event AlertRouteContext) (*AlertRouteDryRunResult, error) {
	routes, err := s.alertRoutes()
	if err != nil {
		return nil, err
	}
	if len(routes) == 0 {
		return s.evaluateLegacyFallback(event)
	}
	return s.evaluateRoutes(event, routes)
}

func (s *AlertService) evaluateLegacyFallback(event AlertRouteContext) (*AlertRouteDryRunResult, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return nil, err
	}
	result := &AlertRouteDryRunResult{
		Event:          event,
		LegacyFallback: true,
	}
	if len(channels) == 0 {
		result.Suppressed = true
		result.SuppressionReason = "no alert channels configured"
		return result, nil
	}
	for _, channel := range channels {
		result.DestinationDecisions = append(result.DestinationDecisions, s.channelDecision(event, db.AlertRoute{}, channel, false, "legacy fallback: no alert routes configured"))
	}
	return result, nil
}

func (s *AlertService) evaluateRoutes(event AlertRouteContext, routes []db.AlertRoute) (*AlertRouteDryRunResult, error) {
	channels, err := s.deliveryChannels()
	if err != nil {
		return nil, err
	}
	channelsByID := map[string]db.AlertChannel{}
	for _, channel := range channels {
		channelsByID[channel.ID] = channel
	}

	result := &AlertRouteDryRunResult{Event: event}
	var suppressingRoute *db.AlertRoute
	for _, route := range routes {
		matched, reasons := routeMatchesEvent(route, event)
		evaluation := AlertRouteEvaluation{
			Route:      route,
			Matched:    matched,
			Suppressed: matched && route.Enabled && route.Suppress,
			Reasons:    reasons,
		}
		if evaluation.Suppressed && suppressingRoute == nil {
			copyRoute := route
			suppressingRoute = &copyRoute
			result.Suppressed = true
			result.SuppressionReason = "alert route suppressed event: " + route.Name
		}
		result.RouteEvaluations = append(result.RouteEvaluations, evaluation)
	}

	for _, evaluation := range result.RouteEvaluations {
		if !evaluation.Matched || !evaluation.Route.Enabled || evaluation.Route.Suppress {
			continue
		}
		for _, channelID := range decodeStringList(evaluation.Route.ChannelIDs) {
			channel, ok := channelsByID[channelID]
			if !ok {
				result.DestinationDecisions = append(result.DestinationDecisions, AlertDestinationDecision{
					RouteID:     evaluation.Route.ID,
					RouteName:   evaluation.Route.Name,
					ChannelID:   channelID,
					ChannelName: channelID,
					ChannelType: "unknown",
					Status:      "suppressed",
					Reason:      "alert route destination missing",
				})
				continue
			}
			suppressedByRoute := suppressingRoute != nil
			reason := "matched alert route: " + evaluation.Route.Name
			if suppressedByRoute {
				reason = "suppressed by alert route: " + suppressingRoute.Name
			}
			result.DestinationDecisions = append(result.DestinationDecisions, s.channelDecision(event, evaluation.Route, channel, suppressedByRoute, reason))
		}
	}

	return result, nil
}

func (s *AlertService) channelDecision(event AlertRouteContext, route db.AlertRoute, channel db.AlertChannel, suppressedByRoute bool, reason string) AlertDestinationDecision {
	decision := AlertDestinationDecision{
		RouteID:     route.ID,
		RouteName:   route.Name,
		ChannelID:   channel.ID,
		ChannelName: channel.Name,
		ChannelType: channel.Type,
		Status:      "pending",
		Reason:      reason,
	}
	switch {
	case suppressedByRoute:
		decision.Status = "suppressed"
	case !channel.Enabled:
		decision.Status = "suppressed"
		decision.Reason = "alert channel disabled"
	case !subscribesToAlertEvent(channel, event.EventType):
		decision.Status = "suppressed"
		decision.Reason = "alert channel is not subscribed to event"
	case s.inCooldown(event.IncidentID, channel.Name, event.EventType, ""):
		decision.Status = "cooldown"
		decision.Reason = "alert cooldown active"
	}
	return decision
}

func routeMatchesEvent(route db.AlertRoute, event AlertRouteContext) (bool, []string) {
	reasons := []string{}
	if !route.Enabled {
		return false, []string{"route disabled"}
	}

	matched := true
	if listContains(decodeAlertRouteEvents(route.EventTypes), event.EventType) {
		reasons = append(reasons, "event matched")
	} else {
		reasons = append(reasons, "event did not match")
		matched = false
	}
	if !filterMatches(route.Severities, event.Severity) {
		reasons = append(reasons, "severity did not match")
		matched = false
	} else if len(decodeStringList(route.Severities)) > 0 {
		reasons = append(reasons, "severity matched")
	}
	if !filterMatches(route.AgentIDs, event.AgentID) {
		reasons = append(reasons, "agent did not match")
		matched = false
	} else if len(decodeStringList(route.AgentIDs)) > 0 {
		reasons = append(reasons, "agent matched")
	}
	if !filterMatches(route.MonitorIDs, event.MonitorID) {
		reasons = append(reasons, "monitor did not match")
		matched = false
	} else if len(decodeStringList(route.MonitorIDs)) > 0 {
		reasons = append(reasons, "monitor matched")
	}
	if !filterMatches(route.MonitorTypes, event.MonitorType) {
		reasons = append(reasons, "monitor type did not match")
		matched = false
	} else if len(decodeStringList(route.MonitorTypes)) > 0 {
		reasons = append(reasons, "monitor type matched")
	}
	return matched, reasons
}

func filterMatches(encoded string, value string) bool {
	values := decodeStringList(encoded)
	if len(values) == 0 {
		return true
	}
	return listContains(values, value)
}

func decodeAlertRouteEvents(encoded string) []string {
	values := decodeStringList(encoded)
	if len(values) == 0 {
		return db.DefaultAlertEvents()
	}
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if db.ValidAlertEvent(value) {
			filtered = append(filtered, value)
		}
	}
	if len(filtered) == 0 {
		return db.DefaultAlertEvents()
	}
	return filtered
}

func decodeStringList(encoded string) []string {
	if strings.TrimSpace(encoded) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(encoded), &values); err != nil {
		return nil
	}
	normalized := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		normalized = append(normalized, value)
	}
	return normalized
}

func listContains(values []string, value string) bool {
	for _, item := range values {
		if item == value {
			return true
		}
	}
	return false
}

func suppressingRouteID(plan *AlertRouteDryRunResult) string {
	for _, evaluation := range plan.RouteEvaluations {
		if evaluation.Suppressed {
			return evaluation.Route.ID
		}
	}
	return ""
}
