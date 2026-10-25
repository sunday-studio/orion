package api

import (
	"encoding/json"
	"net/http"
	"orion/core/internal/db"
	"orion/core/internal/service"
	"sort"
	"strconv"
	"strings"
)

// channelDeliveryResultText is "502", "Timeout", "Sent" for one delivery, from its newest attempt.
func channelDeliveryResultText(d db.AlertDelivery, attempts []db.AlertDeliveryAttempt) string {
	for i := len(attempts) - 1; i >= 0; i-- {
		a := attempts[i]
		if a.AlertDeliveryID != d.ID {
			continue
		}
		if a.HTTPStatus > 0 {
			return httpStatusShort(a.HTTPStatus)
		}
		if a.Error != "" {
			return shortError(a.Error)
		}
		break
	}
	if d.Status == "sent" {
		return "Sent"
	}
	if d.Error != "" {
		return shortError(d.Error)
	}
	return strings.ToUpper(d.Status[:1]) + d.Status[1:]
}

func httpStatusShort(code int) string { return itoa(code) }

func itoa(n int) string { return strconv.Itoa(n) }

// applyChannelHealth derives health, failure streak and the last-delivery text from recent deliveries.
func (s *Server) applyChannelHealth(r *AlertChannelResponse, channel db.AlertChannel) {
	r.HeaderNames = headerNames(channel.HeadersJSON)
	r.SigningSecretMask = maskSecret(channel.WebhookSigningSecret)
	if channel.Type == "email" {
		r.EmailDeliveryMode, r.EmailSubjectPrefix = channel.EmailDeliveryMode, channel.EmailSubjectPrefix
		if r.EmailDeliveryMode == "" {
			r.EmailDeliveryMode = "instant"
		}
	}
	var recent []db.AlertDelivery
	s.db.Preload("Attempts").Where("channel = ? AND status IN ?", channel.Name, []string{"sent", "failed"}).Order("created_at DESC").Limit(50).Find(&recent)
	if len(recent) == 0 {
		r.Health, r.LastDeliveryText = "untested", "Never"
		return
	}
	for _, d := range recent {
		if d.Status != "failed" {
			break
		}
		r.ConsecutiveFailures++
	}
	if r.ConsecutiveFailures == 0 {
		r.Health, r.LastDeliveryText = "healthy", "Sent"
		return
	}
	r.Health = "failing"
	r.LastDeliveryText = itoa(r.ConsecutiveFailures) + " failed, " + channelDeliveryResultText(recent[0], recent[0].Attempts)
}

func headerNames(raw string) []string {
	names := make([]string, 0)
	for name := range service.DecodeChannelHeaders(raw) {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// maskSecret shows the first characters and the last four ("whsec_••••e04b").
func maskSecret(secret string) string {
	if secret == "" {
		return ""
	}
	if len(secret) < 24 {
		// A short, user-chosen secret: reveal at most its last four characters.
		return "••••" + secret[max(len(secret)-4, len(secret)/2):]
	}
	return secret[:6] + "••••" + secret[len(secret)-4:]
}

// applyChannelExtras validates and applies custom headers, email delivery mode and subject prefix.
func applyChannelExtras(channel *db.AlertChannel, request alertChannelRequest) error {
	if request.Headers != nil {
		if err := service.ValidateChannelHeaders(*request.Headers); err != nil {
			return &requestValidationError{message: err.Error()}
		}
		raw, _ := json.Marshal(*request.Headers)
		channel.HeadersJSON = string(raw)
	}
	if request.EmailDeliveryMode != nil {
		mode := strings.TrimSpace(*request.EmailDeliveryMode)
		if mode != "instant" && mode != "digest_15m" {
			return &requestValidationError{message: "email_delivery_mode must be instant or digest_15m"}
		}
		channel.EmailDeliveryMode = mode
	}
	if channel.EmailDeliveryMode == "" {
		channel.EmailDeliveryMode = "instant"
	}
	if request.EmailSubjectPrefix != nil {
		prefix := strings.TrimSpace(*request.EmailSubjectPrefix)
		if len(prefix) > 60 || strings.ContainsAny(prefix, "\r\n") {
			return &requestValidationError{message: "email_subject_prefix is too long or has line breaks"}
		}
		channel.EmailSubjectPrefix = prefix
	}
	return nil
}

func setKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// attemptResultText renders "502 Bad Gateway", "Timeout" or "Sent" for one attempt.
func attemptResultText(a db.AlertDeliveryAttempt) string {
	switch {
	case a.HTTPStatus > 0:
		return itoa(a.HTTPStatus) + " " + http.StatusText(a.HTTPStatus)
	case a.Error != "":
		return shortError(a.Error)
	case a.Status == "sent":
		return "Sent"
	default:
		return ""
	}
}

func enrichAttemptResponse(a db.AlertDeliveryAttempt, r AlertDeliveryAttemptResponse) AlertDeliveryAttemptResponse {
	r.ResultText, r.HTTPStatus = strings.TrimSpace(attemptResultText(a)), a.HTTPStatus
	if a.CompletedAt != nil {
		d := a.CompletedAt.Sub(a.StartedAt).Milliseconds()
		r.DurationMs = &d
	}
	return r
}

// enrichDeliveries adds alert title, INC id, channel id and result text with three bulk lookups.
func (s *Server) enrichDeliveries(rows []AlertDeliveryResponse) {
	if len(rows) == 0 {
		return
	}
	var incidentIDs, routeIDs, channelNames []string
	for _, r := range rows {
		incidentIDs = append(incidentIDs, r.IncidentID)
		if r.RouteID != "" {
			routeIDs = append(routeIDs, r.RouteID)
		}
		channelNames = append(channelNames, r.Channel)
	}
	incidents := map[string]db.Incident{}
	var incs []db.Incident
	s.db.Select("id", "number", "title").Where("id IN ?", incidentIDs).Find(&incs)
	for _, i := range incs {
		incidents[i.ID] = i
	}
	routes := map[string]string{}
	var rts []db.AlertRoute
	if len(routeIDs) > 0 {
		s.db.Select("id", "name").Where("id IN ?", routeIDs).Find(&rts)
	}
	for _, r := range rts {
		routes[r.ID] = r.Name
	}
	channels := map[string]string{}
	var chs []db.AlertChannel
	s.db.Select("id", "name").Where("name IN ?", channelNames).Find(&chs)
	for _, c := range chs {
		channels[c.Name] = c.ID
	}
	for i := range rows {
		r := &rows[i]
		inc := incidents[r.IncidentID]
		r.IncidentDisplayID = incidentDisplayID(inc.Number)
		r.ChannelID = channels[r.Channel]
		r.AlertTitle = firstNonEmptyString(routes[r.RouteID], inc.Title, strings.ReplaceAll(r.EventType, "_", " "))
		if n := len(r.Attempts); n > 0 {
			last := r.Attempts[n-1]
			r.ResultText = last.ResultText
			if last.HTTPStatus > 0 {
				r.ResultText = itoa(last.HTTPStatus) // the list shows "502"; the attempt keeps "502 Bad Gateway"
			}
			r.DurationMs = last.DurationMs
		}
		if r.ResultText == "" {
			switch r.Status {
			case "sent":
				r.ResultText = "Sent"
			case "cooldown":
				r.ResultText = "Cooldown"
			default:
				r.ResultText = shortError(r.Error)
			}
		}
	}
}

func jsonMarshal(v any) (string, error) {
	raw, err := json.Marshal(v)
	return string(raw), err
}

// ruleSummary reads like the Rules list: "When a monitor goes down (critical), notify ops-team, oncall webhook".
func ruleSummary(route db.AlertRoute, channelNames map[string]string) string {
	events := decodeResponseList(route.EventTypes, nil)
	severities := decodeResponseList(route.Severities, nil)
	when := "When an incident opens"
	if len(events) == 1 && events[0] == "incident_resolved" {
		when = "When an incident resolves"
	}
	if len(severities) > 0 {
		when += " (" + strings.Join(severities, ", ") + ")"
	}
	if route.Suppress {
		return when + ", stay quiet"
	}
	var names []string
	for _, id := range decodeResponseList(route.ChannelIDs, nil) {
		if n := channelNames[id]; n != "" {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return when
	}
	return when + ", notify " + strings.Join(names, ", ")
}
