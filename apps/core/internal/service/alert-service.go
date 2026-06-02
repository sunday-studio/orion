package service

import (
	"errors"
	"net/http"
	"orion/core/internal/config"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"time"

	"gorm.io/gorm"
)

const (
	defaultAlertDeliveryMaxAttempts = 3
	defaultAlertDeliveryRetryDelay  = 5 * time.Minute
	alertEventGroupSummary          = "group_summary"
	alertGroupedSummaryPending      = "alert grouped summary pending"
	retiredAlertDeliveryMessage     = "retired alert delivery type suppressed"
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
	logger     *utils.Logger
	cfg        *config.Config
	httpClient *http.Client
}

func NewAlertService(database *gorm.DB, logger *utils.Logger, cfg *config.Config) *AlertService {
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
		return s.db.Model(&db.AlertDelivery{}).Where("id = ?", existing.ID).Updates(map[string]any{
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
	if channel.Type != "webhook" {
		return nil, gorm.ErrRecordNotFound
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
	if err := s.db.Where("type = ?", "webhook").Order("name ASC").Find(&channels).Error; err != nil {
		s.logger.Error("Failed to load alert channels", "error", err)
		return nil, err
	}
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
