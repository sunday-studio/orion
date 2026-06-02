package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"orion/core/internal/db"
	"orion/core/internal/utils"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
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

	if err := db.MigrateWithFiles(database, "migrations", utils.NewLogger()); err != nil {
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
	fmt.Printf("  status pages: %d\n", stats.statusPages)
	fmt.Printf("  status page sections: %d\n", stats.statusPageSections)
	fmt.Printf("  status page components: %d\n", stats.statusPageComponents)
	fmt.Printf("  status page mappings: %d\n", stats.statusPageMappings)
	fmt.Printf("  status page incidents: %d\n", stats.statusPageIncidents)
	fmt.Printf("  status page updates: %d\n", stats.statusPageUpdates)
	fmt.Printf("  status page subscribers: %d\n", stats.statusPageSubscribers)
	fmt.Printf("  status page subscriber component preferences: %d\n", stats.statusPageSubscriberMaps)
	fmt.Printf("  status page deliveries: %d\n", stats.statusPageDeliveries)
}

type seedStats struct {
	agents                   int
	monitors                 int
	agentReports             int
	monitorReports           int
	incidents                int
	incidentEvents           int
	alertDeliveries          int
	rollups                  int
	statusPages              int
	statusPageSections       int
	statusPageComponents     int
	statusPageMappings       int
	statusPageIncidents      int
	statusPageUpdates        int
	statusPageSubscribers    int
	statusPageDeliveries     int
	statusPageSubscriberMaps int
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
	statusPageTables := []string{
		"status_page_subscriber_deliveries",
		"status_page_subscriber_components",
		"status_page_subscribers",
		"status_page_incident_updates",
		"status_page_incidents",
		"status_page_component_mappings",
		"status_page_components",
		"status_page_sections",
		"status_pages",
		"audit_events",
	}
	for _, table := range statusPageTables {
		query := database.Exec(fmt.Sprintf("DELETE FROM %s WHERE id LIKE ?", table), "seed-%")
		if query.Error != nil {
			return query.Error
		}
	}

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

	if err := seedAlertChannels(database, now); err != nil {
		return stats, err
	}

	incidentStats, err := seedIncidents(database, allMonitors, monitorToAgent, monitorToScenario, monitorToTemplate, now)
	if err != nil {
		return stats, err
	}
	stats.incidents += incidentStats.incidents
	stats.incidentEvents += incidentStats.incidentEvents
	stats.alertDeliveries += incidentStats.alertDeliveries

	statusPageStats, err := seedStatusPages(database, allMonitors, now)
	if err != nil {
		return stats, err
	}
	stats.statusPages += statusPageStats.pages
	stats.statusPageSections += statusPageStats.sections
	stats.statusPageComponents += statusPageStats.components
	stats.statusPageMappings += statusPageStats.mappings
	stats.statusPageIncidents += statusPageStats.incidents
	stats.statusPageUpdates += statusPageStats.updates
	stats.statusPageSubscribers += statusPageStats.subscribers
	stats.statusPageDeliveries += statusPageStats.deliveries
	stats.statusPageSubscriberMaps += statusPageStats.subscriberComponents

	return stats, nil
}
