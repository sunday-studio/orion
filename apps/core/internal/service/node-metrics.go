package service

import (
	"fmt"
	"orion/core/internal/db"
	"time"

	"gorm.io/gorm"
)

const nodeSeriesPoints = 12

// NodeMetrics is what the server list and detail show about the latest system reports.
type NodeMetrics struct {
	CPUPercent    *float64
	MemoryPercent *float64
	DiskPercent   *float64
	// CPUSeries holds up to 12 CPU readings, oldest first, for the sparkline.
	CPUSeries     []float64
	NodeVersion   string
	LastReportAt  *time.Time
	UptimeSeconds *uint64
	IP            string
}

// metricsFromReports reads a newest-first slice of reports.
func metricsFromReports(reports []db.NodeReport) NodeMetrics {
	var m NodeMetrics
	if len(reports) == 0 {
		return m
	}
	latest := reports[0]
	cpu, mem, disk := latest.CPU.Data().UsagePercent, latest.Memory.Data().UsedPercent, latest.Disk.Data().UsedPercent
	m.CPUPercent, m.MemoryPercent, m.DiskPercent = &cpu, &mem, &disk
	m.NodeVersion = latest.NodeVersion
	at := latest.CreatedAt
	m.LastReportAt = &at
	up := latest.UptimeSeconds
	m.UptimeSeconds = &up
	m.IP = latest.Location.Data().IP
	n := min(len(reports), nodeSeriesPoints)
	for i := n - 1; i >= 0; i-- {
		m.CPUSeries = append(m.CPUSeries, reports[i].CPU.Data().UsagePercent)
	}
	return m
}

// LatestNodeMetrics loads the newest reports for one node.
func (s *NodeService) LatestNodeMetrics(nodeID string) NodeMetrics {
	var reports []db.NodeReport
	if err := s.db.Where("node_id = ?", nodeID).Order("created_at DESC").Limit(nodeSeriesPoints).Find(&reports).Error; err != nil {
		return NodeMetrics{}
	}
	return metricsFromReports(reports)
}

// CompactDuration renders 41d, 3h, 12m or 30s.
func CompactDuration(d time.Duration) string {
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d/(24*time.Hour)))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d/time.Hour))
	case d >= time.Minute:
		return fmt.Sprintf("%dm", int(d/time.Minute))
	default:
		return fmt.Sprintf("%ds", max(int(d/time.Second), 0))
	}
}

// NodeStatusText is the badge text: "Up 41d", "No report 12m", "Down", "Degraded".
func NodeStatusText(status string, lastSeen time.Time, m NodeMetrics, now time.Time) string {
	switch status {
	case "up":
		if m.UptimeSeconds != nil {
			return "Up " + CompactDuration(time.Duration(*m.UptimeSeconds)*time.Second)
		}
		return "Up"
	case "stale":
		return "No report " + CompactDuration(now.Sub(lastSeen))
	case "down":
		return "Down"
	case "degraded":
		return "Degraded"
	case "maintenance":
		return "Maintenance"
	default:
		return "Unknown"
	}
}

// NodeStatusDetail is the one-line reason next to a problem status ("Disk 91%").
func NodeStatusDetail(status string, m NodeMetrics, reason string) string {
	switch {
	case m.DiskPercent != nil && *m.DiskPercent >= 90:
		return fmt.Sprintf("Disk %d%%", int(*m.DiskPercent+0.5))
	case m.MemoryPercent != nil && *m.MemoryPercent >= 95:
		return fmt.Sprintf("Memory %d%%", int(*m.MemoryPercent+0.5))
	case status != "up" && status != "unknown":
		return reason
	default:
		return ""
	}
}

// GetNodeReportsInRange is GetNodeReportsById limited to since <= created_at < until (either may be nil).
func (s *ReportService) GetNodeReportsInRange(nodeID string, since, until *time.Time, limit int, offset int) ([]db.NodeReport, int64, error) {
	scope := func() *gorm.DB {
		q := s.db.Model(&db.NodeReport{}).Where("node_id = ?", nodeID)
		if since != nil {
			q = q.Where("created_at >= ?", *since)
		}
		if until != nil {
			q = q.Where("created_at < ?", *until)
		}
		return q
	}
	var count int64
	if err := scope().Count(&count).Error; err != nil {
		return nil, 0, err
	}
	var reports []db.NodeReport
	query := scope().Order("created_at DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	if offset > 0 {
		query = query.Offset(offset)
	}
	if err := query.Find(&reports).Error; err != nil {
		return nil, 0, err
	}
	return reports, count, nil
}
