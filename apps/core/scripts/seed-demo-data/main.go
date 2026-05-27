package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/logging"

	"gorm.io/datatypes"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type scenario struct {
	key         string
	name        string
	status      string
	maintenance bool
	stale       bool
	noReports   bool
}

type monitorTemplate struct {
	key         string
	name        string
	kind        string
	description string
	intervalSec int
}

type seedConfig struct {
	dbPath         string
	dataDir        string
	days           int
	agents         int
	reportInterval time.Duration
	resetSeed      bool
	updateSettings bool
}

const defaultAgentCount = 36

type dayCounts struct {
	up       int
	down     int
	degraded int
	unknown  int
}

var scenarios = []scenario{
	{key: "healthy", name: "Healthy Server", status: "up"},
	{key: "degraded", name: "Degraded Server", status: "degraded"},
	{key: "down", name: "Down Server", status: "down"},
	{key: "maintenance", name: "Maintenance Server", status: "maintenance", maintenance: true},
	{key: "stale", name: "Stale Server", status: "stale", stale: true},
	{key: "unknown", name: "Unknown Server", status: "unknown", noReports: true},
	{key: "flapping", name: "Flapping Server", status: "degraded"},
	{key: "tls", name: "TLS Expiring Server", status: "degraded"},
	{key: "resource", name: "Resource Pressure Server", status: "degraded"},
	{key: "alerts", name: "Alert Edge Server", status: "down"},
}

var monitorTemplates = []monitorTemplate{
	{key: "http", name: "HTTP API", kind: "http-healthcheck", description: "HTTP status, latency, body, and regex checks", intervalSec: 60},
	{key: "website", name: "Website", kind: "website", description: "Website check with DNS and TLS metadata", intervalSec: 60},
	{key: "tcp", name: "TCP Port", kind: "tcp", description: "TCP reachability check", intervalSec: 120},
	{key: "resource", name: "Resources", kind: "resource-threshold", description: "CPU, memory, disk, and load thresholds", intervalSec: 60},
	{key: "docker", name: "Docker Container", kind: "docker-container", description: "Docker container status", intervalSec: 120},
	{key: "systemd", name: "systemd Service", kind: "systemd-service", description: "systemd service status", intervalSec: 120},
	{key: "pm2", name: "PM2 Process", kind: "pm2", description: "PM2 process status", intervalSec: 120},
	{key: "command", name: "Command Check", kind: "command", description: "Command exit code, stdout, and stderr", intervalSec: 300},
	{key: "internal", name: "Internal Service", kind: "internal-service", description: "Local ping and process port check", intervalSec: 60},
}

func main() {
	cfg := parseFlags()
	database := openDatabase(cfg)
	sqlDB, err := database.DB()
	if err != nil {
		log.Fatalf("get database handle: %v", err)
	}
	defer sqlDB.Close()

	if err := db.MigrateWithFiles(database, "migrations", logging.NewLogger()); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	if cfg.resetSeed {
		if err := resetSeedData(database); err != nil {
			log.Fatalf("reset seeded data: %v", err)
		}
	}

	stats, err := seed(database, cfg)
	if err != nil {
		log.Fatalf("seed demo data: %v", err)
	}

	fmt.Printf("Seeded Orion demo data into %s\n", cfg.dbPath)
	fmt.Printf("  agents: %d\n", stats.agents)
	fmt.Printf("  monitors: %d\n", stats.monitors)
	fmt.Printf("  agent reports: %d\n", stats.agentReports)
	fmt.Printf("  monitor reports: %d\n", stats.monitorReports)
	fmt.Printf("  incidents: %d\n", stats.incidents)
	fmt.Printf("  incident events: %d\n", stats.incidentEvents)
	fmt.Printf("  alert deliveries: %d\n", stats.alertDeliveries)
	fmt.Printf("  uptime rollups: %d\n", stats.rollups)
}

type seedStats struct {
	agents          int
	monitors        int
	agentReports    int
	monitorReports  int
	incidents       int
	incidentEvents  int
	alertDeliveries int
	rollups         int
}

func parseFlags() seedConfig {
	var cfg seedConfig
	flag.StringVar(&cfg.dataDir, "data-dir", "data", "Core data directory used when -db is empty")
	flag.StringVar(&cfg.dbPath, "db", "", "SQLite database path. Defaults to <data-dir>/orion.db")
	flag.IntVar(&cfg.days, "days", 90, "Number of days of data to generate")
	flag.IntVar(&cfg.agents, "agents", defaultAgentCount, "Number of seed agents to generate")
	flag.DurationVar(&cfg.reportInterval, "report-interval", time.Hour, "Time between generated report samples")
	flag.BoolVar(&cfg.resetSeed, "reset-seed", true, "Delete previous seed-* rows before inserting new data")
	flag.BoolVar(&cfg.updateSettings, "update-settings", true, "Upsert data lifecycle settings for demo data")
	flag.Parse()

	if cfg.dbPath == "" {
		cfg.dbPath = filepath.Join(cfg.dataDir, "orion.db")
	}
	if cfg.days < 1 {
		log.Fatal("-days must be >= 1")
	}
	if cfg.agents < 1 {
		log.Fatal("-agents must be >= 1")
	}
	if cfg.agents < len(scenarios) {
		log.Fatalf("-agents must be >= %d to cover every seed scenario", len(scenarios))
	}
	if cfg.reportInterval < time.Minute {
		log.Fatal("-report-interval must be >= 1m")
	}
	return cfg
}

func openDatabase(cfg seedConfig) *gorm.DB {
	if err := os.MkdirAll(filepath.Dir(cfg.dbPath), 0o755); err != nil {
		log.Fatalf("create database directory: %v", err)
	}
	database, err := gorm.Open(sqlite.Open(cfg.dbPath), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	return database
}

func resetSeedData(database *gorm.DB) error {
	tables := []string{
		"alert_deliveries",
		"incident_events",
		"incidents",
		"monitor_reports",
		"agent_reports",
		"monitors",
		"agents",
	}
	for _, table := range tables {
		query := database.Exec(fmt.Sprintf("DELETE FROM %s WHERE id LIKE ?", table), "seed-%")
		if query.Error != nil {
			return query.Error
		}
	}
	if err := database.Exec("DELETE FROM core_monitor_configs WHERE monitor_id LIKE ?", "seed-%").Error; err != nil {
		return err
	}
	if err := database.Exec("DELETE FROM monitor_uptime_rollups WHERE monitor_id LIKE ?", "seed-%").Error; err != nil {
		return err
	}
	return nil
}

func seed(database *gorm.DB, cfg seedConfig) (seedStats, error) {
	stats := seedStats{}
	now := time.Now().UTC().Truncate(time.Second)
	start := now.AddDate(0, 0, -cfg.days)
	agentCount := cfg.agents
	if agentCount < len(scenarios) {
		agentCount = len(scenarios)
	}

	if cfg.updateSettings {
		if err := seedLifecycleSettings(database, cfg, now); err != nil {
			return stats, err
		}
	}

	allMonitors := []db.Monitor{}
	monitorToAgent := map[string]db.Agent{}
	monitorToScenario := map[string]scenario{}
	monitorToTemplate := map[string]monitorTemplate{}

	for i := 0; i < agentCount; i++ {
		sc := scenarioForIndex(i)
		agent := makeAgent(i, sc, now)
		if err := database.Create(&agent).Error; err != nil {
			return stats, err
		}
		stats.agents++

		if !sc.noReports {
			created, err := seedAgentReports(database, agent, sc, cfg, start, now)
			if err != nil {
				return stats, err
			}
			stats.agentReports += created
		}

		monitors := makeMonitors(agent, sc, now)
		if err := database.Create(&monitors).Error; err != nil {
			return stats, err
		}
		stats.monitors += len(monitors)
		allMonitors = append(allMonitors, monitors...)

		for _, monitor := range monitors {
			monitorToAgent[monitor.ID] = agent
			monitorToScenario[monitor.ID] = sc
			monitorToTemplate[monitor.ID] = templateByMonitor(monitor)
		}
	}

	coreOwner := makeCoreOwner(now)
	if err := database.Create(&coreOwner).Error; err != nil {
		return stats, err
	}
	stats.agents++
	coreMonitor := makeCoreMonitor(coreOwner, now)
	if err := database.Create(&coreMonitor).Error; err != nil {
		return stats, err
	}
	if err := seedCoreMonitorConfig(database, coreMonitor.ID, now); err != nil {
		return stats, err
	}
	stats.monitors++
	allMonitors = append(allMonitors, coreMonitor)
	monitorToAgent[coreMonitor.ID] = coreOwner
	monitorToScenario[coreMonitor.ID] = scenarios[0]
	monitorToTemplate[coreMonitor.ID] = monitorTemplate{
		key:         "core-http",
		name:        "Core HTTP",
		kind:        "http",
		description: "Core-managed HTTP status check",
		intervalSec: 60,
	}

	for _, monitor := range allMonitors {
		if monitor.Lifecycle != "active" {
			continue
		}
		sc := monitorToScenario[monitor.ID]
		tpl := monitorToTemplate[monitor.ID]
		if sc.noReports || strings.Contains(monitor.Name, "Never Reported") {
			continue
		}

		counts, created, err := seedMonitorReports(database, monitor, sc, tpl, cfg, start, now)
		if err != nil {
			return stats, err
		}
		stats.monitorReports += created

		rollups, err := seedRollups(database, monitor.ID, counts, now)
		if err != nil {
			return stats, err
		}
		stats.rollups += rollups
	}

	incidentStats, err := seedIncidents(database, allMonitors, monitorToAgent, monitorToScenario, monitorToTemplate, now)
	if err != nil {
		return stats, err
	}
	stats.incidents += incidentStats.incidents
	stats.incidentEvents += incidentStats.incidentEvents
	stats.alertDeliveries += incidentStats.alertDeliveries

	return stats, nil
}

func scenarioForIndex(i int) scenario {
	if i < len(scenarios) {
		return scenarios[i]
	}
	base := scenarios[i%len(scenarios)]
	base.key = fmt.Sprintf("%s-%02d", base.key, i+1)
	base.name = fmt.Sprintf("%s %02d", base.name, i+1)
	if i%11 == 0 {
		base.maintenance = true
		base.status = "maintenance"
	}
	if i%13 == 0 {
		base.stale = true
		base.status = "stale"
		base.noReports = false
	}
	return base
}

func makeAgent(index int, sc scenario, now time.Time) db.Agent {
	lastSeen := now.Add(-time.Duration(index%8) * time.Minute)
	if sc.stale {
		lastSeen = now.Add(-time.Duration(45+index%72) * time.Minute)
	}
	if sc.noReports {
		lastSeen = now.Add(-time.Duration(index%6) * time.Minute)
	}
	if scenarioBaseKey(sc) == "down" || scenarioBaseKey(sc) == "alerts" {
		lastSeen = now.Add(-time.Duration(index%3) * time.Minute)
	}
	location := db.GeoLocation{
		IP:       fmt.Sprintf("100.64.%d.%d", index/200, 10+index),
		Hostname: fmt.Sprintf("seed-%s.local", sc.key),
		City:     "Home Lab",
		Region:   "Local",
		Country:  "US",
		Loc:      "0,0",
		Org:      "Orion Seed",
		Timezone: "UTC",
	}
	meta := mustJSON(map[string]interface{}{
		"seed":     true,
		"scenario": sc.key,
		"status":   sc.status,
	})
	return db.Agent{
		ID:                       fmt.Sprintf("seed-agent-%02d-%s", index+1, sc.key),
		MachineId:                fmt.Sprintf("seed-machine-%02d-%s", index+1, sc.key),
		Name:                     sc.name,
		OS:                       choose(index%3 == 0, "darwin", "linux"),
		Platform:                 choose(index%3 == 0, "macOS", "ubuntu"),
		KernelVersion:            choose(index%3 == 0, "23.6.0", "6.8.0"),
		Arch:                     choose(index%2 == 0, "arm64", "amd64"),
		Token:                    fmt.Sprintf("seed-token-%02d-%s", index+1, sc.key),
		MaintenanceMode:          sc.maintenance,
		ReportingIntervalSeconds: 60,
		CreatedAt:                now.AddDate(0, 0, -120),
		LastSeen:                 lastSeen,
		Location:                 datatypes.NewJSONType(location),
		Meta:                     meta,
	}
}

func makeMonitors(agent db.Agent, sc scenario, now time.Time) []db.Monitor {
	monitors := make([]db.Monitor, 0, len(monitorTemplates)+3)
	for i, tpl := range monitorTemplates {
		health := currentHealth(sc, tpl)
		var lastSuccess *time.Time
		if health == "up" || health == "degraded" {
			t := now.Add(-time.Duration(i+1) * time.Minute)
			lastSuccess = &t
		}
		description := tpl.description
		monitors = append(monitors, db.Monitor{
			ID:                       fmt.Sprintf("seed-monitor-%s-%s", agent.ID, tpl.key),
			Description:              &description,
			Type:                     tpl.kind,
			Name:                     fmt.Sprintf("%s %s", sc.name, tpl.name),
			AgentID:                  agent.ID,
			LastSuccessfulReportAt:   lastSuccess,
			ReportingIntervalSeconds: tpl.intervalSec,
			ComputedHealth:           health,
			LastHealthComputation:    ptrTime(now.Add(-time.Duration(i) * time.Minute)),
			Lifecycle:                "active",
			Health:                   health,
			IncidentState:            incidentState(health),
			Meta: mustJSON(map[string]interface{}{
				"seed":      true,
				"scenario":  sc.key,
				"monitor":   tpl.key,
				"edge_case": edgeCase(sc, tpl),
			}),
			CreatedAt: now.AddDate(0, 0, -120),
			UpdatedAt: now.Add(-time.Duration(i) * time.Minute),
		})
	}

	disabledDescription := "Disabled monitor for lifecycle filtering"
	deletedDescription := "Deleted monitor for lifecycle filtering"
	monitors = append(monitors,
		db.Monitor{
			ID:          fmt.Sprintf("seed-monitor-%s-disabled", agent.ID),
			Description: &disabledDescription,
			Type:        "http-healthcheck",
			Name:        fmt.Sprintf("%s Disabled Monitor", sc.name),
			AgentID:     agent.ID,
			Lifecycle:   "disabled",
			Health:      "unknown",
			Meta:        mustJSON(map[string]interface{}{"seed": true, "scenario": sc.key, "lifecycle": "disabled"}),
			CreatedAt:   now.AddDate(0, 0, -80),
			UpdatedAt:   now.AddDate(0, 0, -10),
		},
		db.Monitor{
			ID:          fmt.Sprintf("seed-monitor-%s-deleted", agent.ID),
			Description: &deletedDescription,
			Type:        "command",
			Name:        fmt.Sprintf("%s Deleted Monitor", sc.name),
			AgentID:     agent.ID,
			Lifecycle:   "deleted",
			Health:      "unknown",
			Meta:        mustJSON(map[string]interface{}{"seed": true, "scenario": sc.key, "lifecycle": "deleted"}),
			CreatedAt:   now.AddDate(0, 0, -70),
			UpdatedAt:   now.AddDate(0, 0, -20),
			DeletedAt:   now.AddDate(0, 0, -20),
		},
	)

	if sc.noReports {
		neverDescription := "Active monitor with no reports for unknown state"
		monitors = append(monitors, db.Monitor{
			ID:          fmt.Sprintf("seed-monitor-%s-never-reported", agent.ID),
			Description: &neverDescription,
			Type:        "tcp",
			Name:        fmt.Sprintf("%s Never Reported", sc.name),
			AgentID:     agent.ID,
			Lifecycle:   "active",
			Health:      "unknown",
			Meta:        mustJSON(map[string]interface{}{"seed": true, "scenario": sc.key, "edge_case": "never_reported"}),
			CreatedAt:   now.AddDate(0, 0, -20),
			UpdatedAt:   now.AddDate(0, 0, -20),
		})
	}
	return monitors
}

func makeCoreOwner(now time.Time) db.Agent {
	return db.Agent{
		ID:                       "seed-agent-core",
		MachineId:                "core",
		Name:                     "Orion Core",
		OS:                       "linux",
		Platform:                 "orion",
		KernelVersion:            "core",
		Arch:                     "amd64",
		Token:                    "seed-token-core",
		ReportingIntervalSeconds: 60,
		CreatedAt:                now.AddDate(0, 0, -120),
		LastSeen:                 now,
		Meta: mustJSON(map[string]interface{}{
			"seed":  true,
			"owner": "core",
		}),
	}
}

func makeCoreMonitor(agent db.Agent, now time.Time) db.Monitor {
	description := "Core-managed HTTP check seeded for Console owner and source filters"
	lastSuccess := now.Add(-time.Minute)
	return db.Monitor{
		ID:                       "seed-monitor-core-public-api",
		Description:              &description,
		Type:                     "http",
		Name:                     "Core Public API",
		AgentID:                  agent.ID,
		LastSuccessfulReportAt:   &lastSuccess,
		ReportingIntervalSeconds: 60,
		ComputedHealth:           "up",
		LastHealthComputation:    ptrTime(now),
		Lifecycle:                "active",
		Health:                   "up",
		IncidentState:            "unknown",
		Meta: mustJSON(map[string]interface{}{
			"seed":    true,
			"owner":   "core",
			"monitor": "core-http",
		}),
		CreatedAt: now.AddDate(0, 0, -120),
		UpdatedAt: now,
	}
}

func seedCoreMonitorConfig(database *gorm.DB, monitorID string, now time.Time) error {
	return database.Create(&db.CoreMonitorConfig{
		MonitorID:       monitorID,
		Kind:            "http",
		ConfigJSON:      mustJSON(map[string]interface{}{"url": "https://status.example.test/health", "expected_status": 200}),
		SecretRefJSON:   "{}",
		IntervalSeconds: 60,
		TimeoutSeconds:  10,
		NextRunAt:       now.Add(time.Minute),
		CreatedAt:       now,
		UpdatedAt:       now,
	}).Error
}

func seedAgentReports(database *gorm.DB, agent db.Agent, sc scenario, cfg seedConfig, start time.Time, now time.Time) (int, error) {
	reports := make([]db.AgentReport, 0, cfg.days)
	stop := now
	if sc.stale {
		stop = now.Add(-time.Duration(45+len(sc.key)%72) * time.Minute)
	}
	for t := start; !t.After(stop); t = t.Add(cfg.reportInterval) {
		cpu, memory, disk := systemStats(sc, t)
		report := db.AgentReport{
			ID:            fmt.Sprintf("seed-agent-report-%s-%d", agent.ID, t.Unix()),
			AgentID:       agent.ID,
			CreatedAt:     t,
			AgentVersion:  fmt.Sprintf("seed-%s", sc.key),
			ConfigSummary: mustJSON(map[string]interface{}{"monitor_count": len(monitorTemplates), "reporting_interval": cfg.reportInterval.String(), "scenario": sc.key}),
			UptimeSeconds: uint64(math.Max(0, now.Sub(t).Seconds())) + 3600,
			Timestamp:     t.Format(time.RFC3339),
			CPU:           datatypes.NewJSONType(cpu),
			Memory:        datatypes.NewJSONType(memory),
			Disk:          datatypes.NewJSONType(disk),
			Location:      agent.Location,
		}
		reports = append(reports, report)
	}
	return bulkCreate(database, reports, 1000)
}

func seedMonitorReports(database *gorm.DB, monitor db.Monitor, sc scenario, tpl monitorTemplate, cfg seedConfig, start time.Time, now time.Time) (map[string]dayCounts, int, error) {
	reports := make([]db.MonitorReport, 0, cfg.days*24)
	counts := map[string]dayCounts{}
	stop := now
	if sc.stale {
		stop = now.Add(-time.Duration(45+len(sc.key)%72) * time.Minute)
	}
	for t := start; !t.After(stop); t = t.Add(cfg.reportInterval) {
		health := reportHealth(sc, tpl, t, now)
		payload := reportPayload(sc, tpl, health, t, now)
		report := db.MonitorReport{
			ID:          fmt.Sprintf("seed-monitor-report-%s-%d", monitor.ID, t.Unix()),
			MonitorID:   monitor.ID,
			Payload:     mustJSON(payload),
			CollectedAt: t.Format(time.RFC3339),
			Health:      health,
			CreatedAt:   t,
		}
		reports = append(reports, report)
		day := t.Format("2006-01-02")
		c := counts[day]
		switch health {
		case "up":
			c.up++
		case "down":
			c.down++
		case "degraded":
			c.degraded++
		default:
			c.unknown++
		}
		counts[day] = c
	}
	created, err := bulkCreate(database, reports, 1000)
	return counts, created, err
}

func seedRollups(database *gorm.DB, monitorID string, counts map[string]dayCounts, now time.Time) (int, error) {
	rollups := make([]db.MonitorUptimeRollup, 0, len(counts))
	for day, c := range counts {
		total := c.up + c.down + c.degraded + c.unknown
		percent := 0.0
		if total > 0 {
			percent = 100 * float64(c.up) / float64(total)
		}
		rollups = append(rollups, db.MonitorUptimeRollup{
			MonitorID:     monitorID,
			Date:          day,
			UpCount:       c.up,
			DownCount:     c.down,
			DegradedCount: c.degraded,
			UnknownCount:  c.unknown,
			TotalCount:    total,
			UptimePercent: percent,
			CreatedAt:     now,
			UpdatedAt:     now,
		})
	}
	result := database.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "monitor_id"}, {Name: "date"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"up_count", "down_count", "degraded_count", "unknown_count", "total_count", "uptime_percent", "updated_at",
		}),
	}).CreateInBatches(rollups, 1000)
	return len(rollups), result.Error
}

type incidentSeedStats struct {
	incidents       int
	incidentEvents  int
	alertDeliveries int
}

func seedIncidents(database *gorm.DB, monitors []db.Monitor, monitorToAgent map[string]db.Agent, monitorToScenario map[string]scenario, monitorToTemplate map[string]monitorTemplate, now time.Time) (incidentSeedStats, error) {
	stats := incidentSeedStats{}
	statuses := []string{"open", "acknowledged", "resolved"}
	alertStatuses := []string{"pending", "sent", "failed", "suppressed", "cooldown"}
	for _, monitor := range monitors {
		if monitor.Lifecycle != "active" || strings.Contains(monitor.Name, "Never Reported") {
			continue
		}
		sc := monitorToScenario[monitor.ID]
		tpl := monitorToTemplate[monitor.ID]
		if sc.status == "up" {
			continue
		}
		if sc.noReports {
			continue
		}
		agent := monitorToAgent[monitor.ID]
		status := statuses[(stats.incidents)%len(statuses)]
		if sc.status == "down" || sc.status == "alerts" {
			status = "open"
		}
		if sc.maintenance {
			status = "acknowledged"
		}
		openedAt := now.Add(-time.Duration(6+stats.incidents%72) * time.Hour)
		var resolvedAt *time.Time
		if status == "resolved" {
			resolvedAt = ptrTime(openedAt.Add(time.Duration(30+stats.incidents%240) * time.Minute))
		}
		health := currentHealth(sc, tpl)
		if health == "up" {
			health = "degraded"
		}
		incidentID := fmt.Sprintf("seed-incident-%s", monitor.ID)
		incident := db.Incident{
			ID:                 incidentID,
			Status:             status,
			Severity:           severityForHealth(health),
			Title:              fmt.Sprintf("%s is %s", monitor.Name, health),
			AgentID:            agent.ID,
			MonitorID:          monitor.ID,
			OpenedAt:           openedAt,
			ResolvedAt:         resolvedAt,
			LastEventAt:        incidentLastEventAt(resolvedAt, now.Add(-time.Duration(stats.incidents%90)*time.Minute)),
			LatestEvent:        fmt.Sprintf("Seeded %s incident for %s", health, monitor.Name),
			NotificationStatus: alertStatuses[stats.incidents%len(alertStatuses)],
			CreatedAt:          openedAt,
			UpdatedAt:          now,
		}
		if err := database.Create(&incident).Error; err != nil {
			return stats, err
		}
		stats.incidents++

		eventTypes := []string{"incident_opened", "monitor_failed"}
		if status == "resolved" {
			eventTypes = append(eventTypes, "incident_resolved")
		} else {
			eventTypes = append(eventTypes, "alert_rule_matched")
		}
		for i, eventType := range eventTypes {
			event := db.IncidentEvent{
				ID:              fmt.Sprintf("seed-incident-event-%s-%02d", incidentID, i+1),
				IncidentID:      incidentID,
				Type:            eventType,
				Message:         fmt.Sprintf("%s for %s", eventType, monitor.Name),
				MonitorReportID: "",
				CreatedAt:       openedAt.Add(time.Duration(i) * time.Minute),
			}
			if err := database.Create(&event).Error; err != nil {
				return stats, err
			}
			stats.incidentEvents++
		}
		for i, alertStatus := range alertStatuses {
			delivery := db.AlertDelivery{
				ID:         fmt.Sprintf("seed-alert-delivery-%s-%s", incidentID, alertStatus),
				IncidentID: incidentID,
				EventType:  choose(i%2 == 0, "incident_opened", "incident_resolved"),
				Channel:    choose(i%2 == 0, "seed-webhook", "seed-email"),
				Type:       choose(i%2 == 0, "webhook", "email"),
				Status:     alertStatus,
				Error:      choose(alertStatus == "failed", "seeded delivery failure: connection refused", ""),
				CreatedAt:  openedAt.Add(time.Duration(i) * time.Minute),
				UpdatedAt:  openedAt.Add(time.Duration(i+1) * time.Minute),
			}
			if err := database.Create(&delivery).Error; err != nil {
				return stats, err
			}
			stats.alertDeliveries++
		}
		if incident.Status != "resolved" {
			if err := database.Model(&db.Monitor{}).Where("id = ?", monitor.ID).Updates(map[string]interface{}{
				"active_incident_id": incident.ID,
				"incident_state":     incidentState(health),
			}).Error; err != nil {
				return stats, err
			}
		}
	}
	return stats, nil
}

func seedLifecycleSettings(database *gorm.DB, cfg seedConfig, now time.Time) error {
	settings := db.DataLifecycleSettings{
		ID:                1,
		RawReportHotDays:  30,
		ArchiveRawReports: true,
		ArchiveDir:        filepath.Join(filepath.Dir(cfg.dbPath), "archive"),
		RollupsEnabled:    true,
		ArchiveSchedule:   "manual",
		LastRollupRunAt:   ptrTime(now),
		LastArchiveRunAt:  ptrTime(now.Add(-24 * time.Hour)),
		LastArchiveStatus: "success",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	return database.Clauses(clause.OnConflict{UpdateAll: true}).Create(&settings).Error
}

func currentHealth(sc scenario, tpl monitorTemplate) string {
	if sc.maintenance {
		if tpl.key == "resource" || tpl.key == "http" {
			return "degraded"
		}
		return "up"
	}
	if sc.stale {
		return "unknown"
	}
	switch scenarioBaseKey(sc) {
	case "healthy":
		return "up"
	case "degraded", "flapping", "tls", "resource", "maintenance":
		if tpl.key == "website" || tpl.key == "resource" || tpl.key == "http" {
			return "degraded"
		}
		return "up"
	case "down", "alerts":
		if tpl.key == "http" || tpl.key == "website" || tpl.key == "internal" {
			return "down"
		}
		if tpl.key == "resource" {
			return "degraded"
		}
		return "up"
	case "stale", "unknown":
		return "unknown"
	default:
		return "up"
	}
}

func reportHealth(sc scenario, tpl monitorTemplate, t time.Time, now time.Time) string {
	if sc.maintenance {
		if t.After(now.Add(-48 * time.Hour)) {
			return "degraded"
		}
		return "up"
	}
	switch scenarioBaseKey(sc) {
	case "healthy":
		if t.Hour() == 3 && t.Day()%17 == 0 {
			return "degraded"
		}
		return "up"
	case "degraded":
		if tpl.key == "resource" || tpl.key == "http" || t.Hour()%6 == 0 {
			return "degraded"
		}
		return "up"
	case "down":
		if t.After(now.Add(-36 * time.Hour)) {
			return "down"
		}
		if t.Day()%13 == 0 && t.Hour() < 3 {
			return "down"
		}
		return "up"
	case "stale":
		return "up"
	case "flapping":
		if t.Hour()%2 == 0 {
			return "down"
		}
		if t.Hour()%3 == 0 {
			return "degraded"
		}
		return "up"
	case "tls":
		if tpl.key == "website" && t.After(now.AddDate(0, 0, -14)) {
			return "degraded"
		}
		return "up"
	case "resource":
		if tpl.key == "resource" || t.Hour() >= 18 {
			return "degraded"
		}
		return "up"
	case "alerts":
		if t.After(now.Add(-24 * time.Hour)) {
			return "down"
		}
		if t.Day()%9 == 0 {
			return "down"
		}
		return "up"
	default:
		return "up"
	}
}

func reportPayload(sc scenario, tpl monitorTemplate, health string, t time.Time, now time.Time) map[string]interface{} {
	baseKey := scenarioBaseKey(sc)
	payload := map[string]interface{}{
		"seed":        true,
		"scenario":    sc.key,
		"monitor_key": tpl.key,
		"summary":     fmt.Sprintf("%s %s at %s", tpl.kind, health, t.Format(time.RFC3339)),
	}
	latency := 20 + (t.Hour()*7)%400
	payload["latency_ms"] = latency
	if health == "up" {
		payload["ok"] = true
	}
	if health == "degraded" {
		payload["warning"] = "threshold crossed"
		payload["latency_ms"] = latency + 900
	}
	if health == "down" {
		payload["error"] = "seeded outage"
		payload["exit_code"] = 1
	}
	switch tpl.key {
	case "http":
		payload["status_code"] = choose(health == "down", 503, 200)
		payload["expected_status"] = 200
		payload["body_contains"] = choose(health == "down", false, true)
	case "website":
		payload["status_code"] = choose(health == "down", 502, 200)
		payload["dns_lookup_ms"] = 12 + t.Hour()%40
		payload["resolved_ip"] = "203.0.113.10"
		daysRemaining := int(now.Sub(t).Hours() / 24)
		if baseKey == "tls" {
			daysRemaining = 14 - int(now.Sub(t).Hours()/24)
			if daysRemaining < 1 {
				daysRemaining = 1
			}
		} else {
			daysRemaining = 60 + daysRemaining%30
		}
		payload["tls_days_remaining"] = daysRemaining
	case "tcp":
		payload["host"] = "127.0.0.1"
		payload["port"] = 22
		payload["connected"] = health != "down"
	case "resource":
		cpu, memory, disk := systemStats(sc, t)
		payload["cpu_usage_percent"] = cpu.UsagePercent
		payload["memory_used_percent"] = memory.UsedPercent
		payload["disk_used_percent"] = disk.UsedPercent
		payload["load_1"] = cpu.Load1
	case "docker":
		payload["container_name"] = "seed-app"
		payload["state"] = choose(health == "down", "exited", "running")
		payload["restart_count"] = t.Day() % 7
	case "systemd":
		payload["unit"] = "seed.service"
		payload["active_state"] = choose(health == "down", "failed", "active")
	case "pm2":
		payload["app_name"] = "seed-api"
		payload["status"] = choose(health == "down", "errored", "online")
	case "command":
		payload["command"] = "test -f /tmp/seed-ok"
		payload["stdout"] = choose(health == "down", "", "ok")
		payload["stderr"] = choose(health == "down", "missing marker", "")
	case "internal":
		payload["ping_ok"] = health != "down"
		payload["process_port"] = 8999
		payload["process_running"] = health != "down"
	}
	return payload
}

func systemStats(sc scenario, t time.Time) (db.CPUStats, db.MemoryStats, db.DiskStats) {
	wave := float64((t.Hour() + t.Day()) % 24)
	cpuUsed := 15 + wave*2
	memUsed := 40 + wave
	diskUsed := 55 + float64(t.Day()%20)
	switch scenarioBaseKey(sc) {
	case "resource":
		cpuUsed = 88 + float64(t.Hour()%10)
		memUsed = 91
		diskUsed = 93
	case "degraded":
		cpuUsed += 25
		memUsed += 15
	}
	return db.CPUStats{
			Cores:        8,
			UsagePercent: clamp(cpuUsed, 0, 100),
			Load1:        clamp(cpuUsed/20, 0, 12),
			Load5:        clamp(cpuUsed/25, 0, 12),
			Load15:       clamp(cpuUsed/30, 0, 12),
		}, db.MemoryStats{
			TotalBytes:     16 * 1024 * 1024 * 1024,
			UsedBytes:      uint64(memUsed / 100 * 16 * 1024 * 1024 * 1024),
			FreeBytes:      uint64((100 - memUsed) / 100 * 16 * 1024 * 1024 * 1024),
			AvailableBytes: uint64((100 - memUsed) / 100 * 16 * 1024 * 1024 * 1024),
			UsedPercent:    clamp(memUsed, 0, 100),
		}, db.DiskStats{
			TotalBytes:  512 * 1024 * 1024 * 1024,
			UsedBytes:   uint64(diskUsed / 100 * 512 * 1024 * 1024 * 1024),
			FreeBytes:   uint64((100 - diskUsed) / 100 * 512 * 1024 * 1024 * 1024),
			UsedPercent: clamp(diskUsed, 0, 100),
		}
}

func templateByMonitor(monitor db.Monitor) monitorTemplate {
	for _, tpl := range monitorTemplates {
		if strings.HasSuffix(monitor.ID, "-"+tpl.key) {
			return tpl
		}
	}
	return monitorTemplate{key: "unknown", kind: monitor.Type, name: monitor.Name, intervalSec: monitor.ReportingIntervalSeconds}
}

func edgeCase(sc scenario, tpl monitorTemplate) string {
	baseKey := scenarioBaseKey(sc)
	if baseKey == "tls" && tpl.key == "website" {
		return "tls_expiring"
	}
	if baseKey == "resource" && tpl.key == "resource" {
		return "resource_threshold"
	}
	if baseKey == "flapping" {
		return "flapping"
	}
	if sc.stale {
		return "stale_server"
	}
	return sc.status
}

func scenarioBaseKey(sc scenario) string {
	key := sc.key
	if idx := strings.LastIndex(key, "-"); idx > 0 && idx+1 < len(key) {
		suffix := key[idx+1:]
		if _, err := strconv.Atoi(suffix); err == nil {
			return key[:idx]
		}
	}
	return key
}

func incidentState(health string) string {
	switch health {
	case "up", "down", "degraded", "stale":
		return health
	default:
		return "unknown"
	}
}

func severityForHealth(health string) string {
	switch health {
	case "down", "stale":
		return "high"
	case "degraded", "unknown":
		return "medium"
	default:
		return "low"
	}
}

func bulkCreate[T any](database *gorm.DB, rows []T, batchSize int) (int, error) {
	if len(rows) == 0 {
		return 0, nil
	}
	if err := database.CreateInBatches(rows, batchSize).Error; err != nil {
		return 0, err
	}
	return len(rows), nil
}

func mustJSON(value interface{}) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func choose[T any](condition bool, yes T, no T) T {
	if condition {
		return yes
	}
	return no
}

func incidentLastEventAt(resolvedAt *time.Time, fallback time.Time) time.Time {
	if resolvedAt != nil {
		return *resolvedAt
	}
	return fallback
}

func clamp(value float64, min float64, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
