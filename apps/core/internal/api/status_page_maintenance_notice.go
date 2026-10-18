package api

import (
	"context"
	"fmt"
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"strings"
	"time"
)

const maintenanceNoticeLead = 24 * time.Hour

// runMaintenanceNoticeLoop emails subscribers about upcoming windows until ctx ends.
func (s *Server) runMaintenanceNoticeLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if n := s.SendDueMaintenanceNotices(time.Now().UTC()); n > 0 {
				s.logger.Info("Sent maintenance notices", "emails", n)
			}
		}
	}
}

// SendDueMaintenanceNotices emails confirmed subscribers once, up to 24 hours before a window
// that asked for it. A window is marked sent after the pass even if some mails failed, so a bad
// address cannot cause repeats; the per-subscriber delivery rows record failures.
func (s *Server) SendDueMaintenanceNotices(now time.Time) int {
	if s.ensurePublicStatusMailConfigured() != nil {
		return 0
	}
	var windows []db.StatusPageMaintenance
	s.db.Where("notify_subscribers = ? AND notice_sent_at IS NULL AND starts_at > ? AND starts_at <= ?", true, now, now.Add(maintenanceNoticeLead)).Find(&windows)
	sent := 0
	for _, w := range windows {
		sent += s.sendMaintenanceNotice(w, now)
		s.db.Model(&db.StatusPageMaintenance{}).Where("id = ?", w.ID).Update("notice_sent_at", now)
	}
	return sent
}

func (s *Server) sendMaintenanceNotice(w db.StatusPageMaintenance, now time.Time) int {
	var page db.StatusPage
	if err := s.db.Where("id = ? AND visibility IN ? AND published_at IS NOT NULL", w.StatusPageID, []string{statusPageVisibilityPublic, statusPageVisibilityUnlisted}).First(&page).Error; err != nil {
		return 0
	}
	var subscribers []db.StatusPageSubscriber
	s.db.Where("status_page_id = ? AND state = ?", page.ID, statusPageSubscriberStateConfirmed).Find(&subscribers)
	prefs, err := statusPageSubscriberPreferencesBySubscriberID(s.db, subscribers)
	if err != nil {
		return 0
	}
	affected := stringSet(decodeComponentIDs(w.ComponentIDs))
	names := s.statusPageComponentNames(page.ID)
	var list []string
	for id := range affected {
		list = append(list, names[id])
	}
	sent := 0
	for _, sub := range subscribers {
		if !statusPageSubscriberMatchesAffectedComponents(prefs[sub.ID], affected) {
			continue
		}
		destination, err := s.decryptStatusPageSubscriberDestination(sub)
		if err != nil {
			continue
		}
		token, err := utils.GenerateToken()
		if err != nil {
			continue
		}
		sub.UnsubscribeTokenHash = hashStatusPageSubscriberToken(token)
		sub.UnsubscribeTokenVersion++
		if s.db.Save(&sub).Error != nil {
			continue
		}
		unsubscribeURL, err := s.publicStatusPageSubscriberActionURL(page, "unsubscribe", token)
		if err != nil {
			continue
		}
		text := strings.Join([]string{
			fmt.Sprintf("%s: scheduled maintenance", page.Title),
			"",
			w.Title,
			fmt.Sprintf("From %s to %s (UTC)", w.StartsAt.UTC().Format("Jan 2, 15:04"), w.EndsAt.UTC().Format("Jan 2, 15:04")),
			"Affected: " + strings.Join(list, ", "),
			"",
			w.Description,
			"",
			"Unsubscribe: " + unsubscribeURL,
		}, "\n")
		err = s.publicStatusMailSend(publicStatusMailMessage{To: destination, Subject: sanitizePublicStatusMailHeader(fmt.Sprintf("%s: scheduled maintenance, %s", page.Title, w.Title)), Text: text})
		state := statusPageSubscriberDeliveryStateSent
		if err != nil {
			state = statusPageSubscriberDeliveryStateFailed
		} else {
			sent++
		}
		s.recordStatusPageSubscriberDelivery(page.ID, sub.ID, state, "", "")
	}
	return sent
}
