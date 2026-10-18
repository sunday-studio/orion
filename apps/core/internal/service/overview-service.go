package service

import (
	"orion/core/internal/db"
	"orion/core/internal/utils"
	"sort"
	"time"

	"gorm.io/gorm"
)

const (
	overviewServiceLimit  = 8
	overviewHistoryDays   = 90
	overviewRecentLimit   = 5
	overviewOngoingLimit  = 10
	overviewUptimeDays    = 30
	reportingGraceSeconds = 180
)

// Overview is the data behind the Console home screen.
type Overview struct {
	GeneratedAt          time.Time
	State                string // empty | ok | issues
	OngoingIncidentCount int
	HealthIssueCount     int
	FirstRun             OverviewFirstRun
	NeedsYou             *OverviewIncident
	Ongoing              []OverviewIncident
	Recent               []OverviewIncident
	Stats                OverviewStats
	Services             []OverviewService
}

type OverviewFirstRun struct{ HasServers, HasMonitors, HasApps, HasAlertChannels bool }

type OverviewIncident struct {
	ID, Title, Severity, Status, Summary string
	OpenedAt                             time.Time
	ResolvedAt                           *time.Time
	NotifiedAt                           *time.Time
	AcknowledgedAt                       *time.Time
	DurationSeconds                      int64
}

type OverviewStats struct {
	MonitorsUp, MonitorsTotal, ServersReporting, ServersTotal int
	Uptime30dPercent                                          *float64
}

type OverviewService struct {
	Name          string
	AgentID       string
	Status        string // up | degraded | down | unknown
	UptimePercent *float64
	Days          []OverviewDay
}

type OverviewDay struct {
	Date   string
	Status string // up | warn | down | none
}

type OverviewBuilder struct {
	db      *gorm.DB
	logger  *utils.Logger
	reports *ReportService
}

func NewOverviewBuilder(database *gorm.DB, logger *utils.Logger, reports *ReportService) *OverviewBuilder {
	return &OverviewBuilder{db: database, logger: logger, reports: reports}
}

var severityRank = map[string]int{"critical": 0, "high": 1, "medium": 2, "low": 3, "info": 4}

func (b *OverviewBuilder) Build(now time.Time) (*Overview, error) {
	o := &Overview{GeneratedAt: now}

	var agents []db.Agent
	if err := b.db.Where("system = ?", false).Find(&agents).Error; err != nil {
		return nil, err
	}
	var monitors []db.Monitor
	if err := b.db.Where("lifecycle = ?", "active").Find(&monitors).Error; err != nil {
		return nil, err
	}
	var apps, channels int64
	b.db.Model(&db.App{}).Count(&apps)
	b.db.Model(&db.AlertChannel{}).Count(&channels)
	userMonitors := 0
	for _, m := range monitors {
		if !m.System {
			userMonitors++
		}
	}
	o.FirstRun = OverviewFirstRun{HasServers: len(agents) > 0, HasMonitors: userMonitors > 0, HasApps: apps > 0, HasAlertChannels: channels > 0}

	// Stats from cached computed health (kept fresh by the Worker), so Overview stays cheap.
	healthByAgent := map[string]string{}
	for _, m := range monitors {
		h := m.ComputedHealth
		// Core's own monitors (system) can raise health issues for the header chip, but they
		// are not counted as the user's monitors, so a fresh install still reads as empty.
		if h == "down" || h == "degraded" || h == "stale" {
			o.HealthIssueCount++
		}
		if m.System {
			continue
		}
		o.Stats.MonitorsTotal++
		if h == "up" {
			o.Stats.MonitorsUp++
		}
		healthByAgent[m.AgentID] = worseHealth(healthByAgent[m.AgentID], h)
	}
	o.Stats.ServersTotal = len(agents)
	for _, a := range agents {
		grace := time.Duration(max(a.ReportingIntervalSeconds*3, reportingGraceSeconds)) * time.Second
		if now.Sub(a.LastSeen) <= grace {
			o.Stats.ServersReporting++
		}
	}
	o.Stats.Uptime30dPercent = b.uptime(now)

	var active []db.Incident
	if err := b.db.Where("status <> ?", "resolved").Find(&active).Error; err != nil {
		return nil, err
	}
	sort.Slice(active, func(i, j int) bool {
		ri, rj := severityRank[active[i].Severity], severityRank[active[j].Severity]
		if (active[i].Status == "open") != (active[j].Status == "open") {
			return active[i].Status == "open"
		}
		if ri != rj {
			return ri < rj
		}
		return active[i].OpenedAt.Before(active[j].OpenedAt)
	})
	o.OngoingIncidentCount = len(active)
	for i, inc := range active {
		item := b.incident(inc, now)
		if i == 0 && inc.Status == "open" {
			c := item
			b.fillSteps(&c, inc.ID)
			o.NeedsYou = &c
		}
		if i < overviewOngoingLimit {
			o.Ongoing = append(o.Ongoing, item)
		}
	}
	var resolved []db.Incident
	if err := b.db.Where("status = ?", "resolved").Order("resolved_at DESC").Limit(overviewRecentLimit).Find(&resolved).Error; err != nil {
		return nil, err
	}
	for _, inc := range resolved {
		o.Recent = append(o.Recent, b.incident(inc, now))
	}

	switch {
	case !o.FirstRun.HasServers && !o.FirstRun.HasMonitors && o.OngoingIncidentCount == 0 && o.HealthIssueCount == 0:
		o.State = "empty"
	case o.OngoingIncidentCount > 0 || o.HealthIssueCount > 0:
		o.State = "issues"
	default:
		o.State = "ok"
	}
	o.Services = b.services(agents, healthByAgent, now)
	return o, nil
}

func worseHealth(a, b string) string {
	rank := map[string]int{"": 0, "up": 1, "unknown": 2, "stale": 3, "degraded": 4, "down": 5}
	if rank[b] > rank[a] {
		return b
	}
	return a
}

func (b *OverviewBuilder) incident(inc db.Incident, now time.Time) OverviewIncident {
	end := now
	if inc.ResolvedAt != nil {
		end = *inc.ResolvedAt
	}
	return OverviewIncident{ID: inc.ID, Title: inc.Title, Severity: inc.Severity, Status: inc.Status, Summary: inc.LatestEvent,
		OpenedAt: inc.OpenedAt, ResolvedAt: inc.ResolvedAt, DurationSeconds: int64(end.Sub(inc.OpenedAt).Seconds())}
}

// fillSteps adds the Detected / Notified / Acknowledge milestones for the "Needs you now" card.
func (b *OverviewBuilder) fillSteps(item *OverviewIncident, incidentID string) {
	var delivery db.AlertDelivery
	if err := b.db.Where("incident_id = ? AND status = ?", incidentID, "sent").Order("created_at ASC").Limit(1).Find(&delivery).Error; err == nil && delivery.ID != "" {
		t := delivery.CreatedAt
		if delivery.LastAttemptAt != nil {
			t = *delivery.LastAttemptAt
		}
		item.NotifiedAt = &t
	}
	var ack db.IncidentEvent
	if err := b.db.Where("incident_id = ? AND type = ?", incidentID, "incident_acknowledged").Order("created_at ASC").Limit(1).Find(&ack).Error; err == nil && ack.ID != "" {
		t := ack.CreatedAt
		item.AcknowledgedAt = &t
	}
}

func (b *OverviewBuilder) uptime(now time.Time) *float64 {
	since := now.AddDate(0, 0, -overviewUptimeDays).Format("2006-01-02")
	var row struct{ Up, Total int64 }
	if err := b.db.Model(&db.MonitorUptimeRollup{}).Select("COALESCE(SUM(up_count),0) AS up, COALESCE(SUM(total_count),0) AS total").Where("date >= ?", since).Scan(&row).Error; err != nil || row.Total == 0 {
		return nil
	}
	v := 100 * float64(row.Up) / float64(row.Total)
	return &v
}

// services builds the 90-day strip per server, worst first, capped for payload size.
func (b *OverviewBuilder) services(agents []db.Agent, health map[string]string, now time.Time) []OverviewService {
	rank := map[string]int{"down": 0, "degraded": 1, "stale": 2, "unknown": 3, "up": 4, "": 5}
	sort.SliceStable(agents, func(i, j int) bool {
		return rank[health[agents[i].ID]] < rank[health[agents[j].ID]]
	})
	var out []OverviewService
	for _, a := range agents {
		if len(out) >= overviewServiceLimit {
			break
		}
		res, err := b.reports.getAgentUptime(a.ID, "90d", now)
		if err != nil {
			continue
		}
		svc := OverviewService{Name: a.Name, AgentID: a.ID, Status: overviewStatus(health[a.ID])}
		if res.TotalCount > 0 {
			p := res.UptimePercent
			svc.UptimePercent = &p
		}
		for _, d := range res.DailyBuckets {
			status := "none"
			switch {
			case d.Total == 0:
			case d.UptimePercent >= 99.9:
				status = "up"
			case d.UptimePercent >= 95:
				status = "warn"
			default:
				status = "down"
			}
			svc.Days = append(svc.Days, OverviewDay{Date: d.Date, Status: status})
		}
		out = append(out, svc)
	}
	return out
}

func overviewStatus(h string) string {
	switch h {
	case "up", "down", "degraded":
		return h
	case "stale":
		return "degraded"
	default:
		return "unknown"
	}
}
